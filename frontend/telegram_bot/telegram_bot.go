package telegram

import (
	"encoding/json"
	"fmt"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/global"
	"github.com/HouzuoGuo/laitos/httpclient"
	"github.com/HouzuoGuo/laitos/ratelimit"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	ChatTypePrivate   = "private" // Name of the private chat type
	PollIntervalSec   = 5         // Poll for incoming messages every three seconds
	APICallTimeoutSec = 30        // Outgoing API calls are constrained by this timeout
	CommandTimeoutSec = 30        // Command execution is constrained by this timeout
)

// Telegram API entity - user
type APIUser struct {
	ID        uint64 `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	UserName  string `json:"username"`
}

// Telegram API entity - chat
type APIChat struct {
	ID        uint64 `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	UserName  string `json:"username"`
	Type      string `json:"type"`
}

// Telegram API entity - message
type APIMessage struct {
	ID        uint64  `json:"message_id"`
	From      APIUser `json:"from"`
	Chat      APIChat `json:"chat"`
	Timestamp int64   `json:"date"`
	Text      string  `json:"text"`
}

// Telegram API entity - one bot update
type APIUpdate struct {
	ID      uint64     `json:"update_id"`
	Message APIMessage `json:"message"`
}

// Telegram API entity - getUpdates response
type APIUpdates struct {
	OK      bool        `json:"ok"`
	Updates []APIUpdate `json:"result"`
}

// Process feature commands from incoming telegram messages, reply to the chats with command results.
type TelegramBot struct {
	AuthorizationToken string `json:"AuthorizationToken"` // Telegram bot API auth token

	Processor     *common.CommandProcessor `json:"-"` // Feature command processor
	Stop          bool                     `json:"-"` // StartAndBlock function will exit soon after this flag is turned on.
	MessageOffset uint64                   `json:"-"` // Process chat messages arrived after this point
	UserRateLimit *ratelimit.RateLimit     `json:"-"` // Prevent user from flooding bot with new messages
	Logger        global.Logger            `json:"-"` // Logger
}

// Send a text reply to the telegram chat.
func (bot *TelegramBot) ReplyTo(chatID uint64, text string) error {
	resp, err := httpclient.DoHTTP(httpclient.Request{
		Method:     http.MethodPost,
		TimeoutSec: APICallTimeoutSec,
		Body: strings.NewReader(url.Values{
			"chat_id": []string{strconv.FormatUint(chatID, 10)},
			"text":    []string{text},
		}.Encode()),
	}, "https://api.telegram.org/bot%s/sendMessage", bot.AuthorizationToken)
	if err != nil || resp.StatusCode/200 != 1 {
		return fmt.Errorf("failed to send telegram message to chat %d - HTTP %d - %v %s", chatID, resp.StatusCode, err, string(resp.Body))
	}
	return nil
}

// Process incoming chat messages and reply command results to chat initiators.
func (bot *TelegramBot) ProcessMessages(updates APIUpdates) {
	for _, ding := range updates.Updates {
		if bot.MessageOffset <= ding.ID {
			bot.MessageOffset = ding.ID + 1
		}
		// Apply rate limit to the user
		origin := ding.Message.From.UserName
		if origin == "" {
			origin = ding.Message.Chat.UserName
		}
		if !bot.UserRateLimit.Add(origin, true) {
			if err := bot.ReplyTo(ding.Message.Chat.ID, "rate limited"); err != nil {
				bot.Logger.Printf("ProcessMessages", origin, err, "failed to send message reply")
			}
			continue
		}
		// Do not process messages that arrived prior to server startup
		if ding.Message.Timestamp < global.StartupTime.Unix() {
			bot.Logger.Printf("ProcessMessages", origin, nil, "ignore message from \"%s\" that arrived before server started up", ding.Message.Chat.UserName)
			continue
		}
		// Do not process non-private chats
		if ding.Message.Chat.Type != ChatTypePrivate {
			bot.Logger.Printf("ProcessMessages", origin, nil, "ignore non-private chat %d", ding.Message.Chat.ID)
			continue
		}
		// /start is not a command
		if ding.Message.Text == "/start" {
			bot.Logger.Printf("ProcessMessages", origin, nil, "chat %d is started by %s", ding.Message.Chat.ID, ding.Message.Chat.UserName)
			continue
		}
		// Find and run command in background
		go func(ding APIUpdate) {
			result := bot.Processor.Process(feature.Command{TimeoutSec: CommandTimeoutSec, Content: ding.Message.Text})
			if err := bot.ReplyTo(ding.Message.Chat.ID, result.CombinedOutput); err != nil {
				bot.Logger.Printf("ProcessMessages", ding.Message.Chat.UserName, err, "failed to send message reply")
			}
		}(ding)
	}
}

// Immediately begin processing incoming chat messages. Block caller indefinitely.
func (bot *TelegramBot) StartAndBlock() error {
	if errs := bot.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("%+v", errs)
	}
	// Configure rate limit
	bot.UserRateLimit = &ratelimit.RateLimit{
		UnitSecs: PollIntervalSec,
		MaxCount: 1,
		Logger:   bot.Logger,
	}
	bot.UserRateLimit.Initialise()
	// Make a test API call
	testResp, testErr := httpclient.DoHTTP(httpclient.Request{TimeoutSec: APICallTimeoutSec},
		"https://api.telegram.org/bot%s/getMe", bot.AuthorizationToken)
	if testErr != nil || testResp.StatusCode/200 != 1 {
		return fmt.Errorf("TelegramBot.StartAndBlock: test failed - HTTP %d - %v %s", testResp.StatusCode, testErr, string(testResp.Body))
	}
	bot.Logger.Printf("StartAndBlock", "", nil, "going to poll for messages")
	lastIdle := time.Now().Unix()
	for {
		if global.EmergencyLockDown {
			return global.ErrEmergencyLockDown
		}
		if bot.Stop {
			bot.Logger.Printf("StartAndBlock", "", nil, "going to stop now")
			return nil
		}
		// Log a message if the loop has not processed messages for a while (multiplier 200 is an arbitrary choice)
		if idleMax := int64(200 * PollIntervalSec); time.Now().Unix()-lastIdle > idleMax {
			bot.Logger.Printf("Loop", "", nil, "has been idling for %d seconds", idleMax)
			lastIdle = time.Now().Unix()
		}
		// Poll for new messages
		updatesResp, updatesErr := httpclient.DoHTTP(httpclient.Request{TimeoutSec: APICallTimeoutSec},
			"https://api.telegram.org/bot%s/getUpdates?offset=%s", bot.AuthorizationToken, bot.MessageOffset)
		var newMessages APIUpdates
		if updatesErr != nil || updatesResp.StatusCode/200 != 1 {
			bot.Logger.Printf("Loop", "", updatesErr, "failed to poll due to HTTP %d %s", updatesResp.StatusCode, string(updatesResp.Body))
			goto sleepAndContinue
		}
		// Deserialise new messages
		if err := json.Unmarshal(updatesResp.Body, &newMessages); err != nil {
			bot.Logger.Printf("Loop", "", err, "failed to decode response JSON")
			goto sleepAndContinue
		}
		if !newMessages.OK {
			bot.Logger.Printf("Loop", "", nil, "API response is not OK - %s", string(updatesResp.Body))
			goto sleepAndContinue
		}
		// Process new messages
		if len(newMessages.Updates) > 0 {
			lastIdle = time.Now().Unix()
			bot.ProcessMessages(newMessages)
		}
	sleepAndContinue:
		time.Sleep(PollIntervalSec * time.Second)
	}
}
