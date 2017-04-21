package smtpd

import (
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/frontend/mailp"
	"net/smtp"
	"strings"
	"testing"
	"time"
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
		PerIPLimit:    10,
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

	/*
		SMTP daemon is expected to start in a few seconds, it may take a short while because
		the daemon has to figure out its public IP address.
	*/
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	var stoppedNormally bool
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stoppedNormally = true
	}()
	time.Sleep(3 * time.Second) // this really should be env.HTTPPublicIPTimeout * time.Second
	// Try to exceed rate limit
	testMessage := "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body"
	success := 0
	for i := 0; i < 100; i++ {
		if err := smtp.SendMail("127.0.0.1:61358", nil, "ClientFrom@localhost", []string{"ClientTo@howard.name"}, []byte(testMessage)); err == nil {
			success++
		}
	}
	if success < 5 || success > 15 {
		t.Fatal("delivered", success)
	}
	time.Sleep(RateLimitIntervalSec * time.Second)
	// Send an ordinary mail to the daemon
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body"
	if err := smtp.SendMail("127.0.0.1:61358", nil, "ClientFrom@localhost", []string{"ClientTo@example.com"}, []byte(testMessage)); err != nil {
		t.Fatal(err)
	}
	// Send a mail that does not belong to this server's domain
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body"
	if err := smtp.SendMail("127.0.0.1:61358", nil, "ClientFrom@localhost", []string{"ClientTo@not-my-domain"}, []byte(testMessage)); strings.Index(err.Error(), "Bad address") == -1 {
		t.Fatal(err)
	}
	// Try run a command via email
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: command subject\r\n\r\nverysecret.s echo hi"
	if err := smtp.SendMail("127.0.0.1:61358", nil, "ClientFrom@localhost", []string{"ClientTo@howard.name"}, []byte(testMessage)); err != nil {
		t.Fatal(err)
	}
	t.Log("Check howard@localhost and root@localhost mailbox")
	// Daemon must stop in a second
	daemon.Stop()
	time.Sleep(1 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	daemon.Stop()
	daemon.Stop()
}
