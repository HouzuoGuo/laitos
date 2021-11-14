package toolbox

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMessageBank_Store(t *testing.T) {
	bank := &MessageBank{}
	if !bank.IsConfigured() {
		t.Fatal("not configured")
	}
	if err := bank.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := bank.SelfTest(); err != nil {
		t.Fatal(err)
	}

	if err := bank.Store("bad tag", MessageDirectionIncoming, time.Now(), "haha"); err == nil || !strings.Contains(err.Error(), "unrecognised tag") {
		t.Fatal(err)
	}
	if err := bank.Store(MessageBankTagTTN, "bad dir", time.Now(), "haha"); err == nil || !strings.Contains(err.Error(), "unrecognised direction") {
		t.Fatal(err)
	}
	if err := bank.Store(MessageBankTagTTN, MessageDirectionIncoming, time.Now(), nil); err == nil || !strings.Contains(err.Error(), "must not be nil") {
		t.Fatal(err)
	}

	// Default, incoming direction.
	now := time.Now()
	if err := bank.Store(MessageBankTagDefault, MessageDirectionIncoming, now, "alpha"); err != nil {
		t.Fatal(err)
	}
	if err := bank.Store(MessageBankTagDefault, MessageDirectionIncoming, now, "beta"); err != nil {
		t.Fatal(err)
	}
	if err := bank.Store(MessageBankTagDefault, MessageDirectionOutgoing, now, "charlie"); err != nil {
		t.Fatal(err)
	}
	gotMessages := bank.Get(MessageBankTagDefault, MessageDirectionIncoming)
	wantMessages := []Message{
		{Time: now, Content: "alpha"},
		{Time: now, Content: "beta"},
	}
	if !reflect.DeepEqual(wantMessages, gotMessages) {
		t.Fatalf("%+v", gotMessages)
	}

	// TTN, outgoing direction.
	if err := bank.Store(MessageBankTagTTN, MessageDirectionOutgoing, now, "delta"); err != nil {
		t.Fatal(err)
	}
	gotMessages = bank.Get(MessageBankTagTTN, MessageDirectionOutgoing)
	if !reflect.DeepEqual([]Message{{Time: now, Content: "delta"}}, gotMessages) {
		t.Fatalf("%+v", gotMessages)
	}

	// Overwrite older message entries.
	for i := 0; i < MessageBankMaxMessagesPerDirection-1; i++ {
		if err := bank.Store(MessageBankTagDefault, MessageDirectionIncoming, now, "charlie"); err != nil {
			t.Fatal(err)
		}
	}
	gotMessages = bank.Get(MessageBankTagDefault, MessageDirectionIncoming)
	if len(gotMessages) != MessageBankMaxMessagesPerDirection || !reflect.DeepEqual(Message{Time: now, Content: "beta"}, gotMessages[0]) {
		t.Fatalf("%d, %+v", len(gotMessages), gotMessages[0])
	}
}

func TestMessageBank_Execute(t *testing.T) {
	bank := &MessageBank{}
	if err := bank.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Process an invalid command.
	result := bank.Execute(context.Background(), Command{Content: "not-a-valid-command"})
	if result.Error == nil {
		t.Fatalf("%+v", result)
	}
	// Store two messages.
	for _, content := range []string{"s default in alpha", "s default in beta"} {
		result := bank.Execute(context.Background(), Command{Content: content})
		if result.Error != nil || result.Output != MessageBankDefaultStoreResponse {
			t.Fatalf("%+v", result)
		}
	}
	// Retrieve both messages.
	result = bank.Execute(context.Background(), Command{Content: "g default in"})
	t.Logf("%+v", result)
	if result.Error != nil || !strings.Contains(result.Output, "alpha") || !strings.Contains(result.Output, "beta") {
		t.Fatalf("%+v", result)
	}
	// Store a message in the outbound direction.
	distantTimestamp := time.Date(2002, 03, 04, 05, 06, 07, 00, time.UTC)
	if err := bank.Store(MessageBankTagDefault, MessageDirectionOutgoing, distantTimestamp, "charlie"); err != nil {
		t.Fatal(err)
	}
	result = bank.Execute(context.Background(), Command{Content: "s default in delta"})
	if result.Error != nil || result.Output != fmt.Sprintf("Stored. Last outbound message was: %s charlie", distantTimestamp.Format(MessageBankDateFormat)) {
		t.Fatalf("%+v", result)
	}
}
