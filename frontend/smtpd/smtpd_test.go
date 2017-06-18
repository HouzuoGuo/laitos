package smtpd

import (
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/frontend/mailp"
	"strings"
	"testing"
)

func TestSMTPD_StartAndBlock(t *testing.T) {
	goodMailer := email.Mailer{
		MailFrom: "howard@localhost",
		MTAHost:  "127.0.0.1",
		MTAPort:  25,
	}
	daemon := SMTPD{
		ListenAddress: "127.0.0.1",
		ListenPort:    61358, // hard coded port is a random choice
		PerIPLimit:    0,
		MailProcessor: &mailp.MailProcessor{
			CommandTimeoutSec: 10,
			Processor:         common.GetTestCommandProcessor(),
			ReplyMailer:       goodMailer,
		},
		ForwardMailer: goodMailer,
	}
	// Must not initialise if mail processor reply mailer is not there
	daemon.MailProcessor.ReplyMailer.MTAHost = ""
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "reply mailer") {
		t.Fatal(err)
	}
	daemon.MailProcessor.ReplyMailer = goodMailer
	// Must not initialise if per IP limit is too small
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "PerIPLimit") {
		t.Fatal(err)
	}
	daemon.PerIPLimit = 10
	// Must not intialise if forward-to addresses are not there
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "forward address") {
		t.Fatal(err)
	}
	daemon.ForwardTo = []string{"root", "howard@example.com"}
	// Must not initialise if forward mailer is not there
	daemon.ForwardMailer.MTAHost = ""
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "forward mailer") {
		t.Fatal(err)
	}
	// Must not initialise if my domain names are not there
	daemon.ForwardMailer = email.Mailer{
		MailFrom: "howard@abc",
		MTAHost:  "a.b.c.d",
		MTAPort:  61358,
	}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "my domain names") {
		t.Fatal(err)
	}
	daemon.MyDomains = []string{"example.com", "howard.name"}
	// Must not initialise if forward mailer is myself
	daemon.ForwardMailer = email.Mailer{
		MailFrom: "howard@localhost",
		MTAHost:  "127.0.0.1",
		MTAPort:  61358,
	}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "forward MTA") {
		t.Fatal(err)
	}
	daemon.ForwardMailer = goodMailer
	// Must not initialise if mail processor reply mailer is myself
	daemon.MailProcessor.ReplyMailer = email.Mailer{
		MailFrom: "howard@localhost",
		MTAHost:  "127.0.0.1",
		MTAPort:  61358,
	}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "reply MTA") {
		t.Fatal(err)
	}
	daemon.MailProcessor.ReplyMailer = goodMailer
	// One of the forward addresses does not have at sign
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "at sign") {
		t.Fatal(err)
	}
	// One of the forward addresses loops back to server domain
	daemon.ForwardTo = []string{"howard@example.com", "howard@other.com"}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "loop back") {
		t.Fatal(err)
	}
	daemon.ForwardTo = []string{"howard@localhost"}

	TestSMTPD(&daemon, t)
}
