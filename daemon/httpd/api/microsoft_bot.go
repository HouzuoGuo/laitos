package api

import (
	"bytes"
	"encoding/json"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/toolbox"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	MicrosoftBotAPITimeoutSec     = 30 // MicrosoftBotAPITimeoutSec is the timeout seconds for outgoing HTTP calls.
	MicrosoftBotCommandTimeoutSec = 30 // Command execution for Microsoft bot is constrained by this timeout.
)

// MicrosoftBotJwt is a JWT returned by Microsoft bot framework.
type MicrosoftBotJwt struct {
	TokenType    string    `json:"token_type"`     // TokenType should always be "Bearer".
	ExpiresIn    int       `json:"expires_in"`     // ExpiresIn is the number of seconds till expiry.
	ExtExpiresIn int       `json:"ext_expires_in"` // ExtExpiresIn is not relevant, I do not know what it does.
	AccessToken  string    `json:"access_token"`   // AccessToken is the JWT.
	ExpiresAt    time.Time `json:"-"`              // ExpiresAt is the exact time of expiry, calculated by RetrieveJWT function.
}

// HandleMicrosoftBot serves a chat bot endpoint for Microsoft bot framework.
type HandleMicrosoftBot struct {
	ClientAppID     string `json:"ClientAppID"`     // ClientAppID is the bot's "app ID".
	ClientAppSecret string `json:"ClientAppSecret"` // ClientAppSecret is the bot's application "password".

	latestJwtMutex *sync.Mutex     // latestJwtMutex protects latestJWT from concurrent access.
	latestJWT      MicrosoftBotJwt // latestJWT is the last retrieved JWT
}

// RetrieveJWT asks Microsoft for a new JWT.
func (hand *HandleMicrosoftBot) RetrieveJWT() (jwt MicrosoftBotJwt, err error) {
	var httpResp inet.HTTPResponse
	httpResp, err = inet.DoHTTP(inet.HTTPRequest{
		Method:      http.MethodPost,
		ContentType: "application/x-www-form-urlencoded",
		TimeoutSec:  MicrosoftBotAPITimeoutSec,
		Body: strings.NewReader(url.Values{
			"grant_type":    []string{"client_credentials"},
			"client_id":     []string{hand.ClientAppID},
			"client_secret": []string{hand.ClientAppSecret},
			"scope":         []string{"https://api.botframework.com/.default"},
		}.Encode()),
	}, "https://login.microsoftonline.com/botframework.com/oauth2/v2.0/token")
	if err != nil {
		return
	}
	if err = json.Unmarshal(httpResp.Body, &jwt); err != nil {
		return
	}
	// Exact time of expiry is simply time now + validity in seconds (ExpiresIn)
	jwt.ExpiresAt = time.Now().Add(time.Duration(jwt.ExpiresIn) * time.Second)
	return
}

// MicrosoftBotIncomingConversation is the construct of property "conversation" of MicrosoftBotIncomingChat.
type MicrosoftBotIncomingConversation struct {
	ID      string          `json:"id"`
	IsGroup json.RawMessage `json:"isGroup"`
	Name    json.RawMessage `json:"name"`
}

// MicrosoftBotIncomingChat is an "Activity object" carried by incoming chat initiated by a user to bot.
type MicrosoftBotIncomingChat struct {
	Conversation MicrosoftBotIncomingConversation `json:"conversation"` // Conversation will go into reply's "conversation" property.
	From         json.RawMessage                  `json:"from"`         // From will go into reply's "recipient" property.
	Locale       json.RawMessage                  `json:"locale"`       // Locale will go into reply's "locale" property.
	Recipient    json.RawMessage                  `json:"recipient"`    // Recipient will go into reply's "from" property.
	ID           json.RawMessage                  `json:"id"`           // ID will go into reply's "id" property.
	Text         string                           `json:"text"`         // Text is the content of incoming chat message.
	ServiceURL   string                           `json:"serviceUrl"`   // ServiceURL is the prefix name of endpoint to send chat reply to.
	Timestamp    string                           `json:"timestamp"`    // Timestamp is the timestamp of incoming chat message.
}

// MicrosoftBotReply is a message reply to be sent to user who initiated chat with bot.
type MicrosoftBotReply struct {
	Conversation MicrosoftBotIncomingConversation `json:"conversation"` // Conversation value comes from MicrosoftBotIncomingChat.
	From         json.RawMessage                  `json:"from"`         // From value comes from MicrosoftBotIncomingChat's "Recipient".
	Locale       json.RawMessage                  `json:"locale"`       // Locale value comes from MicrosoftBotIncomingChat.
	Recipient    json.RawMessage                  `json:"recipient"`    // Recipient value comes from MicrosoftBotIncomingChat's "From".
	ReplyToId    json.RawMessage                  `json:"replyToId"`    // ReplyToId  value comes from MicrosoftBotIncomingChat's "ID".
	Type         string                           `json:"type"`         // Type must be "message".
	Text         string                           `json:"text"`         // Text is the bot's response text.
}

func (hand *HandleMicrosoftBot) MakeHandler(logger misc.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	hand.latestJwtMutex = new(sync.Mutex)
	fun := func(w http.ResponseWriter, r *http.Request) {
		// Deserialise chat message from incoming request
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warningf("HandleMicrosoftBot", "", err, "failed to read incoming chat HTTP request")
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		var incoming MicrosoftBotIncomingChat
		if err := json.Unmarshal(body, &incoming); err != nil {
			logger.Warningf("HandleMicrosoftBot", "", err, "failed to interpret incoming chat request as JSON")
			http.Error(w, "failed to read request body in JSON", http.StatusBadRequest)
			return
		}
		// In the background, process the chat message and formulate a response.
		go func() {
			// If JWT has not been acquired or is outdated, request a new one.
			var jwtCopy MicrosoftBotJwt
			hand.latestJwtMutex.Lock()
			if hand.latestJWT.AccessToken == "" || time.Now().After(hand.latestJWT.ExpiresAt) {
				jwt, err := hand.RetrieveJWT()
				logger.Printf("HandleMicrosoftBot", "", err, "attempted to renew JWT")
				if err != nil {
					return
				}
				// Remember the latest valid JWT for consecutive chat responses
				hand.latestJWT = jwt
				jwtCopy = jwt
			} else {
				jwtCopy = hand.latestJWT
			}
			hand.latestJwtMutex.Unlock()

			// Sometimes a chat app establishes a conversation without any content, just ignore it.
			if incoming.Text == "" {
				return
			}

			// Only process an incoming message if it arrived after server started up
			messageTime, err := time.ParseInLocation("2006-01-02T15:04:05.999999999Z", incoming.Timestamp, time.UTC)
			if err != nil {
				logger.Warningf("HandleMicrosoftBot", "", err, "failed to parse timestamp \"%s\" from incoming message", incoming.Timestamp)
				return
			}
			if !messageTime.After(misc.StartupTime.UTC()) {
				logger.Warningf("HandleMicrosoftBot", "", err, "ignoring message \"%s\" that arrived before server started up", incoming.Text)
				return
			}

			// Process feature command from incoming chat text
			result := cmdProc.Process(toolbox.Command{TimeoutSec: MicrosoftBotCommandTimeoutSec, Content: incoming.Text})

			// Most of the reply properties are directly copied from incoming request
			var reply MicrosoftBotReply
			reply.Conversation = incoming.Conversation
			reply.From = incoming.Recipient
			reply.Locale = incoming.Locale
			reply.Recipient = incoming.From
			reply.ReplyToId = incoming.ID
			reply.Type = "message"
			reply.Text = result.CombinedOutput
			replyBody, err := json.Marshal(reply)
			if err != nil {
				logger.Warningf("HandleMicrosoftBot", "", err, "failed to serialise chat reply")
				return
			}
			// Send away the reply
			_, err = inet.DoHTTP(inet.HTTPRequest{
				Method:      http.MethodPost,
				ContentType: "application/json",
				TimeoutSec:  MicrosoftBotAPITimeoutSec,
				Header:      http.Header{"Authorization": []string{"Bearer " + jwtCopy.AccessToken}},
				Body:        bytes.NewReader(replyBody),
			}, incoming.ServiceURL+"/v3/conversations/%s/activities/%s", incoming.Conversation.ID, incoming.ID)
			if err != nil {
				logger.Warningf("HandleMicrosoftBot", "", err, "failed to send chat reply")
				return
			}
		}()

	}
	return fun, nil
}

func (hand *HandleMicrosoftBot) GetRateLimitFactor() int {
	return 1
}
