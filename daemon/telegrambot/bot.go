package telegrambot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const (
	ChatTypePrivate   = "private" // Name of the private chat type
	APICallTimeoutSec = 30        // Outgoing API calls are constrained by this timeout
	CommandTimeoutSec = 30        // Command execution is constrained by this timeout

	/*
		PollIntervalSecMin and PollIntervalSecMax together determine the range of random number of seconds to wait between
		each message polling attempt. The randomness helps multiple laitos instances to poll messages simultaneously
		without starving any specific instance.
	*/
	PollIntervalSecMin = 2
	PollIntervalSecMax = 5
)

// Telegram API entity - user
type APIUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	UserName  string `json:"username"`
}

// Telegram API entity - chat
type APIChat struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	UserName  string `json:"username"`
	Type      string `json:"type"`
}

// Telegram API entity - message
type APIMessage struct {
	ID        int64   `json:"message_id"`
	From      APIUser `json:"from"`
	Chat      APIChat `json:"chat"`
	Timestamp int64   `json:"date"`
	Text      string  `json:"text"`
}

// Telegram API entity - one bot update
type APIUpdate struct {
	ID      int64      `json:"update_id"`
	Message APIMessage `json:"message"`
}

// Telegram API entity - getUpdates response
type APIUpdates struct {
	OK      bool        `json:"ok"`
	Updates []APIUpdate `json:"result"`
}

// Process feature commands from incoming telegram messages, reply to the chats with command results.
type Daemon struct {
	AuthorizationToken string                    `json:"AuthorizationToken"` // Telegram bot API auth token
	PerUserLimit       int                       `json:"PerUserLimit"`       // PerUserLimit determines how many messages may be processed per chat at regular interval
	Processor          *toolbox.CommandProcessor `json:"-"`                  // Feature command processor

	messageOffset int64           // Process chat messages arrived after this point
	userRateLimit *misc.RateLimit // Prevent user from flooding bot with new messages
	cancelFunc    context.CancelFunc
	logger        lalog.Logger
}

func (bot *Daemon) Initialise() error {
	if bot.PerUserLimit < 1 {
		bot.PerUserLimit = 2 // reasonable for personal use
	}
	bot.logger = lalog.Logger{ComponentName: "telegrambot", ComponentID: []lalog.LoggerIDField{{Key: "PerUserLimit", Value: bot.PerUserLimit}}}
	if bot.Processor == nil || bot.Processor.IsEmpty() {
		return fmt.Errorf("telegrambot.Initialise: command processor and its filters must be configured")
	}
	bot.Processor.SetLogger(bot.logger)
	if errs := bot.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("telegrambot.Initialise: %+v", errs)
	}
	if bot.AuthorizationToken == "" {
		return errors.New("telegrambot.Initialise: AuthorizationToken must not be empty")
	}
	// Configure rate limit
	bot.userRateLimit = &misc.RateLimit{
		UnitSecs: PollIntervalSecMax,
		MaxCount: bot.PerUserLimit,
		Logger:   bot.logger,
	}
	bot.userRateLimit.Initialise()
	return nil
}

// Send a text reply to the telegram chat.
func (bot *Daemon) ReplyTo(chatID int64, text string) error {
	resp, err := inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method:     http.MethodPost,
		TimeoutSec: APICallTimeoutSec,
		Body: strings.NewReader(url.Values{
			"chat_id": []string{strconv.FormatInt(chatID, 10)},
			"text":    []string{text},
		}.Encode()),
	}, "https://api.telegram.org/bot%s/sendMessage", bot.AuthorizationToken)
	if err != nil || resp.StatusCode/200 != 1 {
		return fmt.Errorf("telegrambot.ReplyTo: failed to reply to %d - HTTP %d - %v %s", chatID, resp.StatusCode, err, string(resp.Body))
	}
	return nil
}

// Process incoming chat messages and reply command results to chat initiators.
func (bot *Daemon) ProcessMessages(ctx context.Context, updates APIUpdates) {
	for _, ding := range updates.Updates {
		// Put processing duration (including API time) into statistics
		beginTimeNano := time.Now().UnixNano()
		if bot.messageOffset <= ding.ID {
			bot.messageOffset = ding.ID + 1
		}
		// Apply rate limit to the user
		origin := ding.Message.From.UserName
		if origin == "" {
			origin = ding.Message.Chat.UserName
		}
		if !bot.userRateLimit.Add(origin, true) {
			if err := bot.ReplyTo(ding.Message.Chat.ID, "rate limited"); err != nil {
				bot.logger.Warning("ProcessMessages", origin, err, "failed to reply rate limited response")
			}
			continue
		}
		// Do not process messages that arrived prior to server startup
		if ding.Message.Timestamp < misc.StartupTime.Unix() {
			bot.logger.Warning("ProcessMessages", origin, nil, "ignore message from \"%s\" that arrived before server started up", ding.Message.Chat.UserName)
			continue
		}
		// Do not process non-private chats
		if ding.Message.Chat.Type != ChatTypePrivate {
			bot.logger.Warning("ProcessMessages", origin, nil, "ignore non-private chat %d", ding.Message.Chat.ID)
			continue
		}
		// /start is not a command
		if ding.Message.Text == "/start" {
			bot.logger.Info("ProcessMessages", origin, nil, "chat %d is started by %s", ding.Message.Chat.ID, ding.Message.Chat.UserName)
			continue
		}
		// Find and run command in background
		go func(ding APIUpdate, beginTimeNano int64) {
			result := bot.Processor.Process(ctx, toolbox.Command{
				DaemonName: "telegrambot",
				ClientTag:  ding.Message.Chat.UserName,
				TimeoutSec: CommandTimeoutSec,
				Content:    ding.Message.Text,
			}, true)
			if err := bot.ReplyTo(ding.Message.Chat.ID, result.CombinedOutput); err != nil {
				bot.logger.Warning("ProcessMessages", ding.Message.Chat.UserName, err, "failed to send message reply")
			}
			misc.TelegramBotStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
		}(ding, beginTimeNano)
	}
}

// Immediately begin processing incoming chat messages. Block caller indefinitely.
func (bot *Daemon) StartAndBlock() error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	bot.cancelFunc = cancelFunc
	/*
		Make a test API call to verify the correctness of authorization token. This test call must not return in case of
		IO error or unexpected HTTP response status. As of 2017-11-26, status 404 is the only indication of incorrect
		authorization token for now.
	*/
	testResp, testErr := inet.DoHTTP(context.TODO(), inet.HTTPRequest{TimeoutSec: APICallTimeoutSec},
		"https://api.telegram.org/bot%s/getMe", bot.AuthorizationToken)
	if testErr == nil && testResp.StatusCode == http.StatusNotFound {
		return errors.New("telegrambot.StartAndBlock: test call failed due to HTTP 404, is the AuthorizationToken correct?")
	}
	bot.logger.Info("StartAndBlock", "", nil, "going to poll for messages")
	periodicFunc := func(ctx context.Context, _, _ int) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		// Poll for new messages
		updatesResp, updatesErr := inet.DoHTTP(context.TODO(), inet.HTTPRequest{TimeoutSec: APICallTimeoutSec},
			"https://api.telegram.org/bot%s/getUpdates?offset=%s", bot.AuthorizationToken, bot.messageOffset)
		if updatesErr == nil {
			updatesErr = updatesResp.Non2xxToError()
		}
		var newMessages APIUpdates
		if updatesErr != nil {
			/*
				Before February 2018, if two or more program instances poll for messages at the same time, Telegram API
				would not complain, but lately it begins to complain:
				"terminated by other getUpdates request; make sure that only one bot instance is running"
				despite that all program instances continue to function well and successfully poll all messages in its
				next attempt. Therefore, suppress the log message in this case, and randomise the number of seconds to
				wait between polling attempts.
			*/
			if updatesResp.StatusCode != http.StatusConflict {
				bot.logger.Warning("Loop", "", updatesErr, "failed to poll due to HTTP error")
			}
			return nil
		}
		// Deserialise new messages
		if err := json.Unmarshal(updatesResp.Body, &newMessages); err != nil {
			bot.logger.Warning("Loop", "", err, "failed to decode response JSON")
			return nil
		}
		if !newMessages.OK {
			bot.logger.Warning("Loop", "", nil, "API response is not OK - %s", string(updatesResp.Body))
			return nil
		}
		// Process new messages
		if len(newMessages.Updates) > 0 {
			bot.ProcessMessages(context.TODO(), newMessages)
		}
		return nil
	}
	periodic := &misc.Periodic{
		LogActorName: bot.logger.ComponentName,
		Interval:     time.Duration(PollIntervalSecMin+rand.Intn(PollIntervalSecMax-PollIntervalSecMin)) * time.Second,
		MaxInt:       1,
		Func:         periodicFunc,
	}
	periodic.Start(ctx)
	return periodic.WaitForErr()
}

// Stop previously started message handling loop.
func (bot *Daemon) Stop() {
	bot.cancelFunc()
}

// Run unit tests on telegram bot. See TestSMTPD_StartAndBlock for bot setup.
func TestTelegramBot(bot *Daemon, t testingstub.T) {
	// Well then it is really difficult to test the chat routine
	// So I am going to only going to start the daemon using invalid configuration, which is definitely failing.
	if err := bot.StartAndBlock(); err == nil || !strings.Contains(err.Error(), "AuthorizationToken") {
		t.Fatal(err)
	}
	// Repeatedly stopping the daemon should have no negative consequence
	bot.Stop()
	bot.Stop()
}
