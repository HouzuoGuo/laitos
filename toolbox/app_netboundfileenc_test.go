package toolbox

import (
	"context"
	"fmt"
	"testing"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/netboundfileenc"
	"github.com/HouzuoGuo/laitos/netboundfileenc/unlocksvc"
)

func TestNetBoundFileEncryption(t *testing.T) {
	nbe := NetBoundFileEncryption{PasswordRegister: netboundfileenc.NewPasswordRegister(2, 10, lalog.DefaultLogger)}
	if err := nbe.Initialise(); err != nil {
		t.Fatal(err)
	}
	if !nbe.IsConfigured() {
		t.Fatal("nbe does not require configuration hence it should always be in a configured state")
	}
	if err := nbe.SelfTest(); err != nil {
		t.Fatal(err)
	}

	// Post an unlocking intent and then check outstanding intents
	randChallenge := netboundfileenc.GetRandomChallenge()
	_, err := nbe.PostUnlockIntent(context.Background(), &unlocksvc.PostUnlockIntentRequest{Identification: &unlocksvc.UnlockAttemptIdentification{
		HostName:        "test1.example.com",
		PID:             1234,
		RandomChallenge: randChallenge,
	}})
	if err != nil {
		t.Fatal(err)
	}
	result := nbe.Execute(context.Background(), Command{Content: ""})
	expectedOutput := fmt.Sprintf("%s\t1234\ttest1.example.com\t\n", randChallenge)
	if result.Error != nil || result.Output != expectedOutput {
		t.Fatalf("Err: %v\nOutput: %v", result.Error, result.Output)
	}

	// Fulfil the outstanding intent by offering the password to it
	result = nbe.Execute(context.Background(), Command{Content: "malformed-input"})
	if result.Error == nil {
		t.Fatal("should have returned an error along with a hint of correct command syntax")
	}
	result = nbe.Execute(context.Background(), Command{Content: fmt.Sprintf("%s %s", randChallenge, "your-password")})
	if result.Error != nil || result.Output != "OK" {
		t.Fatalf("Err: %v\nOutput: %v", result.Error, result.Output)
	}

	if len(nbe.GetOutstandingIntents()) != 0 {
		t.Fatalf("%+v", nbe.GetOutstandingIntents())
	}
}
