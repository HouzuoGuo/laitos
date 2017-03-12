package telegram

import (
	"encoding/json"
	"fmt"
	"github.com/HouzuoGuo/websh/feature"
	"github.com/HouzuoGuo/websh/frontend/common"
	"github.com/HouzuoGuo/websh/ratelimit"
	"github.com/kotarock/Lookatme.OpenAPI/httpclient"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	ChatTypePrivate   = "private" // Name of the private chat type
	PollIntervalSec   = 5         // Poll for incoming messages every three seconds
	CommandTimeoutSec = 30        // By default, commands from chat run using 30 seconds timeout.
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
	Processor          *common.CommandProcessor `json:"-"`                  // Feature command processor
	AuthorizationToken string                   `json:"AuthorizationToken"` // Telegram bot API auth token
	Stop               bool                     `json:"-"`                  // StartAndBlock function will exit soon after this flag is turned on.
	MessageOffset      uint64                   `json:"-"`                  // Process chat messages arrived after this point
	UserRateLimit      *ratelimit.RateLimit     `json:"-"`                  // Prevent user from flooding bot with new messages
}

// Send a text reply to the telegram chat.
func (bot *TelegramBot) ReplyTo(chatID uint64, text string) error {
	resp, err := httpclient.DoHTTP(httpclient.HTTPRequest{
		Method: http.MethodPost,
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
		if bot.MessageOffset < ding.ID {
			bot.MessageOffset = ding.ID + 1
		}
		// Apply rate limit to the user
		origin := ding.Message.From.UserName
		if origin == "" {
			origin = ding.Message.Chat.UserName
		}
		if !bot.UserRateLimit.Add(origin, true) {
			if err := bot.ReplyTo(ding.Message.Chat.ID, "rate limited"); err != nil {
				log.Printf("TELEGRAMBOT: failed to reply to %s - %v", origin, err)
			}
			continue
		}
		// Do not process non-private chats
		if ding.Message.Chat.Type != ChatTypePrivate {
			log.Printf("TELEGRAMBOT: ignore non-private chat %d from %s", ding.Message.Chat.ID, ding.Message.Chat.UserName)
			continue
		}
		// /start is not a command
		if ding.Message.Text == "/start" {
			log.Printf("TELEGRAMBOT: chat %d started by %s - %s", ding.Message.Chat.ID, ding.Message.Chat.UserName, ding.Message.From.UserName)
			continue
		}
		// Find and run command in background
		go func(ding APIUpdate) {
			result := bot.Processor.Process(feature.Command{TimeoutSec: CommandTimeoutSec, Content: ding.Message.Text})
			if err := bot.ReplyTo(ding.Message.Chat.ID, result.CombinedOutput); err != nil {
				log.Printf("TELEGRAMBOT: failed to reply to %s - %v", origin, err)
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
	}
	// Make a test API call
	testResp, testErr := httpclient.DoHTTP(httpclient.HTTPRequest{}, "https://api.telegram.org/bot%s/getMe", bot.AuthorizationToken)
	if testErr != nil || testResp.StatusCode/200 != 1 {
		return fmt.Errorf("TelegramBot.StartAndBlock: test failed - HTTP %d - %v %s", testResp.StatusCode, testErr, string(testResp.Body))
	}
	lastIdle := time.Now().Unix()
	for {
		if bot.Stop {
			log.Print("TELEGRAMBOT: signaled to stop")
			return nil
		}
		// Log a message if the loop has not processed messages for a while (multiplier 200 is an arbitrary choice)
		if idleMax := int64(200 * PollIntervalSec); time.Now().Unix()-lastIdle > idleMax {
			log.Printf("TELEGRAMBOT: has been idling for %d seconds", idleMax)
			lastIdle = time.Now().Unix()
		}
		// Poll for new messages
		updatesResp, updatesErr := httpclient.DoHTTP(httpclient.HTTPRequest{}, "https://api.telegram.org/bot%s/getUpdates?offset=%s", bot.AuthorizationToken, bot.MessageOffset)
		var newMessages APIUpdates
		if updatesErr != nil || updatesResp.StatusCode/200 != 1 {
			log.Printf("TELEGRAMBOT: failed to poll - HTTP %d - %v - %s", updatesResp.StatusCode, updatesErr, string(updatesResp.Body))
			goto sleepAndContinue
		}
		// Deserialise new messages
		if err := json.Unmarshal(updatesResp.Body, &newMessages); err != nil {
			log.Printf("TELEGRAMBOT: failed to decode response JSON - %v", err)
			goto sleepAndContinue
		}
		if !newMessages.OK {
			log.Printf("TELEGRAMBOT: API response is not OK - %s", string(updatesResp.Body))
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
