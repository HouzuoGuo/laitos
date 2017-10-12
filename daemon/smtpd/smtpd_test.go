package smtpd

import (
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/inet"
	"strings"
	"testing"
)

func TestSMTPD_StartAndBlock(t *testing.T) {
	goodMailer := inet.MailClient{
		MailFrom: "howard@localhost",
		MTAHost:  "127.0.0.1",
		MTAPort:  25,
	}
	daemon := Daemon{
		Address:           "127.0.0.1",
		Port:              61358, // hard coded port is a random choice
		PerIPLimit:        0,
		ForwardMailClient: goodMailer,
	}
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
	daemon.ForwardMailClient.MTAHost = ""
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "forward mail client") {
		t.Fatal(err)
	}
	// Must not initialise if my domain names are not there
	daemon.ForwardMailClient = inet.MailClient{
		MailFrom: "howard@abc",
		MTAHost:  "a.b.c.d",
		MTAPort:  61358,
	}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "my domain names") {
		t.Fatal(err)
	}
	daemon.MyDomains = []string{"example.com", "howard.name"}
	// Must not initialise if forward mailer is myself
	daemon.ForwardMailClient = inet.MailClient{
		MailFrom: "howard@localhost",
		MTAHost:  "127.0.0.1",
		MTAPort:  61358,
	}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "forward MTA") {
		t.Fatal(err)
	}
	daemon.ForwardMailClient = goodMailer
	// One of the forward addresses loops back to server domain
	daemon.ForwardTo = []string{"howard@example.com", "howard@other.com"}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "loop back") {
		t.Fatal(err)
	}
	// One of the forward addresses does not have at sign
	daemon.ForwardTo = []string{"howard@another-domain.com", "howard"}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "at sign") {
		t.Fatal(err)
	}
	daemon.ForwardTo = []string{"howard@localhost"}
	// Parameters are now satisfied, however there is not a mail command runner. This should not raise an error
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}

	// SMTPD is also responsible for initialising its mail command runner if it is present
	daemon.CommandRunner = &mailcmd.CommandRunner{
		Processor:       nil,
		ReplyMailClient: inet.MailClient{},
	}
	// Even though command runner is assigned, SMTPD should continue to initialise without command runner if command runner does not have config.
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// SMTPD should not receive init error from command processor
	daemon.CommandRunner.Processor = common.GetInsaneCommandProcessor()
	if err := daemon.Initialise(); !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal(err)
	}
	// Give it a good command processor
	daemon.CommandRunner.Processor = common.GetTestCommandProcessor()

	// Must not initialise if mail processor reply mailer is myself
	daemon.CommandRunner.ReplyMailClient = inet.MailClient{
		MailFrom: "howard@localhost",
		MTAHost:  "127.0.0.1",
		MTAPort:  61358,
	}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "reply MTA") {
		t.Fatal(err)
	}
	// Must not initialise if mail processor reply mailer is not there
	daemon.CommandRunner.ReplyMailClient.MTAHost = ""
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "reply mailer") {
		t.Fatal(err)
	}
	daemon.CommandRunner.ReplyMailClient = goodMailer

	TestSMTPD(&daemon, t)
}
