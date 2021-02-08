package netboundfileenc

import (
	"context"
	"testing"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/netboundfileenc/unlocksvc"
)

func TestGetRandomChallenge(t *testing.T) {
	r1 := GetRandomChallenge()
	r2 := GetRandomChallenge()
	if len(r1) < 12 || len(r1) > MaxRandomChallengeLen || r1 == r2 {
		t.Fatal(r1, r2)
	}
}

func TestPasswordRegister_GetOutstandingIntents(t *testing.T) {
	reg := NewPasswordRegister(2, 10, lalog.DefaultLogger)
	// Post two intents and fulfil the first one
	intents := []struct {
		hostName  string
		PID       uint64
		challenge string
	}{
		{"example-name1", 12345, GetRandomChallenge()},
		{"example-name2", 12345, GetRandomChallenge()},
	}
	for _, intent := range intents {
		_, err := reg.PostUnlockIntent(context.Background(), &unlocksvc.PostUnlockIntentRequest{
			Identification: &unlocksvc.UnlockAttemptIdentification{
				HostName:        intent.hostName,
				PID:             intent.PID,
				RandomChallenge: intent.challenge,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if !reg.FulfilIntent(intents[0].challenge, "example-password") {
		t.Fatal("should have found the corresponding outstanding intent")
	}

	// Retrieve the second intent which is yet to be fulfilled
	outstandingIntents := reg.GetOutstandingIntents()
	outstandingIntent := outstandingIntents[intents[1].challenge]
	if len(outstandingIntents) != 1 || outstandingIntent == nil {
		t.Fatalf("%+v", outstandingIntents)
	}
	if outstandingIntent.HostName != intents[1].hostName || outstandingIntent.PID != intents[1].PID || outstandingIntent.RandomChallenge != intents[1].challenge {
		t.Fatalf("%+v", outstandingIntent)
	}
}

func TestPasswordRegister_GetUnlockPassword(t *testing.T) {
	reg := NewPasswordRegister(2, 10, lalog.DefaultLogger)
	challengeStrs := []string{GetRandomChallenge(), GetRandomChallenge(), GetRandomChallenge()}
	// Post intents
	intents := []struct {
		hostName  string
		PID       uint64
		challenge string
	}{
		{"example-name1", 12345, challengeStrs[0]},
		{"example-name2", 23456, challengeStrs[1]},
		{"example-name3", 34567, challengeStrs[2]},
		// Repeatedly posted intents shall still get its unlocking password at the end
		{"example-name3", 34567, challengeStrs[2]},
	}
	for _, intent := range intents {
		_, err := reg.PostUnlockIntent(context.Background(), &unlocksvc.PostUnlockIntentRequest{
			Identification: &unlocksvc.UnlockAttemptIdentification{
				HostName:        intent.hostName,
				PID:             intent.PID,
				RandomChallenge: intent.challenge,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// There is capacity for two intents, the first one posted has been evicted.
	if outstandingIntents := reg.GetOutstandingIntents(); len(outstandingIntents) != 2 {
		t.Fatalf("%+v", outstandingIntents)
	}

	// Fulfil two intents
	if !reg.FulfilIntent(intents[1].challenge, "password1") {
		t.Fatal("should have found the corresponding outstanding intent")
	}
	if !reg.FulfilIntent(intents[2].challenge, "password2") {
		t.Fatal("should have found the corresponding outstanding intent")
	}

	// Retireve passwords and verify
	passwordRetrieval := []struct {
		challenge, password string
	}{
		{intents[1].challenge, "password1"},
		{intents[2].challenge, "password2"},
	}
	for _, retrieval := range passwordRetrieval {
		resp, err := reg.GetUnlockPassword(context.Background(), &unlocksvc.GetUnlockPasswordRequest{
			Identification: &unlocksvc.UnlockAttemptIdentification{
				HostName:        "does-not-matter",
				PID:             0,
				RandomChallenge: retrieval.challenge,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !resp.Exists || resp.Password != retrieval.password {
			t.Fatalf("%+v", resp)
		}
	}

	// Passwords are no longer available after successful retrieval
	if outstandingIntents := reg.GetOutstandingIntents(); len(outstandingIntents) != 0 {
		t.Fatalf("%+v", outstandingIntents)
	}
	if len(reg.FulfilledIntents) != 0 {
		t.Fatalf("%+v", reg.FulfilledIntents)
	}
	if reg.IntentsChallenge.Len() != 0 {
		t.Fatalf("%+s", reg.IntentIdentifications)
	}

	// Must fail to retrieve password if the challenge string mismatches
	resp, err := reg.GetUnlockPassword(context.Background(), &unlocksvc.GetUnlockPasswordRequest{
		Identification: &unlocksvc.UnlockAttemptIdentification{
			HostName:        "does-not-matter",
			PID:             0,
			RandomChallenge: "wrong-challenge-string",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Exists || resp.Password != "" {
		t.Fatalf("%+v", resp)
	}
}

func TestPasswordRegister_FulfilNonExistingIntent(t *testing.T) {
	reg := NewPasswordRegister(2, 10, lalog.DefaultLogger)
	if reg.FulfilIntent("non-existing", "pass") {
		t.Fatal("should not have allowed non-existing intent to be fulfilled")
	}
}
