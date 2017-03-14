package smtpd

import (
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

func TestSMTPD_StartAndBlock(t *testing.T) {
	daemon := SMTPD{
		ListenAddress: "127.0.0.1",
		ListenPort:    61358, // hard coded port is a random choice
		IPLimit:       3,
		Processor:     &common.CommandProcessor{},
		ForwardTo:     []string{"howard@localhost", "root@localhost"},
		ForwardMailer: email.Mailer{},
	}
	// Must not initialise if command processor is insane
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal("did not error due to insane CommandProcessor")
	}
	daemon.Processor = common.GetTestCommandProcessor()
	// Must not initialise if mailer is not there
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "forward mailer") {
		t.Fatal("did not error due to bad mailer")
	}
	// Must not initialise if mailer is myself
	daemon.ForwardMailer = email.Mailer{
		MailFrom: "howard@localhost",
		MTAHost:  "127.0.0.1",
		MTAPort:  61358,
	}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "myself") {
		t.Fatal("did not error due to bad mailer")
	}
	// Finally a good mailer
	daemon.ForwardMailer = email.Mailer{
		MailFrom: "howard@localhost",
		MTAHost:  "127.0.0.1",
		MTAPort:  25,
	}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	/*
		SMTP daemon is expected to start in a few seconds, it may take a short while because
		the daemon has to figure out its public IP address.
	*/
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(3 * time.Second) // this really should be env.HTTPPublicIPTimeout * time.Second
	// Send an ordinary mail to the daemon
	testMessage := "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body"
	if err := smtp.SendMail("127.0.0.1:61358", nil, "ClientFrom@localhost", []string{"ClientTo@localhost"}, []byte(testMessage)); err != nil {
		if err != nil {
			t.Fatal(err)
		}
	}
	// Try run a command via email
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: command subject\r\n\r\nverysecret.s echo hi"
	if err := smtp.SendMail("127.0.0.1:61358", nil, "ClientFrom@localhost", []string{"ClientTo@localhost"}, []byte(testMessage)); err != nil {
		if err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(3 * time.Second)
	t.Log("Check howard@localhost and root@localhost mailbox")
}
