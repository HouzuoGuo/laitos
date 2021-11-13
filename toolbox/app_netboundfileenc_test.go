package toolbox

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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
	id := &unlocksvc.UnlockAttemptIdentification{
		HostName:        "test1.example.com",
		PID:             46318972,
		RandomChallenge: randChallenge,
		UserID:          56134789,
		UptimeSec:       13256784,
	}
	_, err := nbe.PostUnlockIntent(context.Background(), &unlocksvc.PostUnlockIntentRequest{Identification: id})
	if err != nil {
		t.Fatal(err)
	}
	result := nbe.Execute(context.Background(), Command{Content: ""})
	if result.Error != nil {
		t.Fatalf("Err: %v\nOutput: %v", result.Error, result.Output)
	}
	for _, needle := range []string{
		id.HostName,
		strconv.Itoa(int(id.PID)),
		randChallenge,
		strconv.Itoa(int(id.UserID)),
		strconv.Itoa(int(id.UptimeSec)),
	} {
		if !strings.Contains(result.Output, needle) {
			t.Fatalf("Cannot find needle %q, got %q", needle, result.Output)
		}
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
