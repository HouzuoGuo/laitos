package toolbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"
)

const (
	MessageBankMaxTextLen              = 1024
	MessageBankMaxMessagesPerDirection = 10
	MessageDirectionIncoming           = "in"
	MessageDirectionOutgoing           = "out"
	MessageBankTagDefault              = "default"
	MessageBankTagTTN                  = "TTN"
	MessageBankDefaultStoreResponse    = "message has been stored"
)

var (
	allTags = map[string]bool{MessageBankTagDefault: true, MessageBankTagTTN: true}

	MessageBankRegexStore = regexp.MustCompile(`s[^\w]+([\w]+)[^\w]+([\w]+)[^\w]+(.*)`)
	MessageBankRegexGet   = regexp.MustCompile(`g[^\w]+([\w]+)[^\w]+([\w]+)`)

	MessageBankDateFormat = "20060102T150405Z"
)

// Message is a timestamped text message.
type Message struct {
	Time    time.Time
	Content interface{}
}

// MessageBank stores two-way text messages for on-demand retrieval.
type MessageBank struct {
	mutex       *sync.Mutex
	allMessages map[string]map[string][]Message
}

// IsConfigured always returns true.
func (*MessageBank) IsConfigured() bool {
	return true
}

// SelfTest always returns nil.
func (*MessageBank) SelfTest() error {
	return nil
}

// Store memorises the message of arbitrary type. If the maximum number of
// messages is reached for the combination of tag and direction, then the oldest
// message will be evicted prior to storing this message.
func (bank *MessageBank) Store(tag, direction string, timestamp time.Time, content interface{}) error {
	if exists := allTags[tag]; !exists {
		return fmt.Errorf("Store: unrecognised tag %q", tag)
	}
	if direction != MessageDirectionIncoming && direction != MessageDirectionOutgoing {
		return fmt.Errorf("Store: unrecognised direction %q", direction)
	}
	if content == nil {
		return errors.New("Store: content must not be nil")
	}
	bank.mutex.Lock()
	defer bank.mutex.Unlock()
	dirMessages, exists := bank.allMessages[tag]
	if !exists {
		dirMessages = make(map[string][]Message)
	}
	messages, exists := dirMessages[direction]
	if !exists {
		messages = make([]Message, 0, MessageBankMaxMessagesPerDirection)
	}
	if len(messages) == MessageBankMaxMessagesPerDirection {
		// Evict the oldest message.
		messages = messages[1:]
	}
	messages = append(messages, Message{Time: timestamp, Content: content})
	dirMessages[direction] = messages
	bank.allMessages[tag] = dirMessages
	return nil
}

// Get retrieves the messages currently stored under the specified tag and
// direction.
func (bank *MessageBank) Get(tag, direction string) []Message {
	bank.mutex.Lock()
	defer bank.mutex.Unlock()
	if dirMessages, exists := bank.allMessages[tag]; !exists {
		return []Message{}
	} else if messages, exists := dirMessages[direction]; !exists {
		return []Message{}
	} else {
		return messages
	}
}

// Initialise initialises the internal states of the app.
func (bank *MessageBank) Initialise() error {
	bank.allMessages = make(map[string]map[string][]Message)
	bank.mutex = new(sync.Mutex)
	return nil
}

// Trigger returns the string ".b".
func (*MessageBank) Trigger() Trigger {
	return ".b"
}

func (bank *MessageBank) Execute(ctx context.Context, cmd Command) *Result {
	if errResult := cmd.Trim(); errResult != nil {
		return errResult
	}
	// By chance, each of the two regular expressions has a distinct number of
	// capture groups.
	storeParams := MessageBankRegexStore.FindStringSubmatch(cmd.Content)
	if len(storeParams) == 4 {
		if err := bank.Store(storeParams[1], storeParams[2], time.Now(), storeParams[3]); err != nil {
			return &Result{Error: err}
		}
		// Respond with the latest outbound messages if it is sufficiently new.
		messages := bank.Get(storeParams[1], MessageDirectionOutgoing)
		if len(messages) > 0 {
			latest := messages[len(messages)-1]
			return &Result{Output: fmt.Sprintf("Stored. Last outbound message was: %s %+v", latest.Time.UTC().Format(MessageBankDateFormat), latest.Content)}
		}
		return &Result{Output: MessageBankDefaultStoreResponse}
	} else if getParams := MessageBankRegexGet.FindStringSubmatch(cmd.Content); len(getParams) == 3 {
		var output bytes.Buffer
		for _, message := range bank.Get(getParams[1], getParams[2]) {
			output.WriteString(fmt.Sprintf("%s %+v\n", message.Time.UTC().Format(MessageBankDateFormat), message.Content))
		}
		return &Result{Output: output.String()}
	} else {
		return &Result{Error: errors.New(".b s Tag Direction Text | g Tag Direction")}
	}
}
