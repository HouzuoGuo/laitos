package main

import (
	"testing"
	"time"
)

func TestMailNotification(t *testing.T) {
	mailer := Mailer{}
	if mailer.IsEnabled() {
		t.Fatal("not right")
	}

	mailer.Recipients = []string{"root@localhost"}
	mailer.MailFrom = "root@localhost"
	mailer.MTAAddressPort = "localhost:25"
	if !mailer.IsEnabled() {
		t.Fatal("not right")
	}
	mailer.SendNotification("test command", "test output")
	// Should be long enough for mail to be delivered
	time.Sleep(1 * time.Second)
}
