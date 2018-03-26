package smtpd

import (
	"crypto/tls"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/inet"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"testing"
)

func TestSMTPD_StartAndBlock(t *testing.T) {
	daemon := Daemon{
		ForwardMailClient: inet.MailClient{
			MailFrom: "howard@localhost",
			MTAHost:  "smtp.example.com",
			MTAPort:  25,
		},
	}
	// Test missing mandatory parameters
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "forward address") {
		t.Fatal(err)
	}
	daemon.ForwardTo = []string{"root@example.netnetnet", "howard@example.comcomcom"}

	daemon.ForwardMailClient.MTAHost = ""
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "forward mail client") {
		t.Fatal(err)
	}
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
		MTAPort:  daemon.Port,
	}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "forward MTA") {
		t.Fatal(err)
	}
	// One of the forward addresses loops back to server domain
	daemon.ForwardMailClient = inet.MailClient{
		MailFrom: "howard@localhost",
		MTAHost:  "127.0.0.1",
		MTAPort:  25252, // avoid triggering the init error of looping mails back to daemon itself
	}
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

	// Test default settings
	if err := daemon.Initialise(); err != nil || daemon.Address != "0.0.0.0" || daemon.Port != 25 || daemon.PerIPLimit != 8 {
		t.Fatalf("%+v %+v", err, daemon)
	}

	// Prepare settings for tests
	daemon.ForwardMailClient = inet.MailClient{
		MailFrom: "howard@localhost",
		MTAHost:  "127.0.0.1",
		MTAPort:  25, // allow developer to inspect result by running a postfix
	}
	daemon.Address = "127.0.0.1"
	daemon.Port = 61358   // do not loop back to myself
	daemon.PerIPLimit = 5 // limit must be high enough to tolerate consecutive mail tests
	// If parameter values are satisfactory, daemon should properly and successfully initialise even without the presence of a mail command runner
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
		MTAPort:  daemon.Port,
	}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "reply MTA") {
		t.Fatal(err)
	}
	// Must not initialise if mail processor reply mailer is not there
	daemon.CommandRunner.ReplyMailClient.MTAHost = ""
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "reply mailer") {
		t.Fatal(err)
	}
	daemon.CommandRunner.ReplyMailClient = inet.MailClient{
		MailFrom: "howard@localhost",
		MTAHost:  "127.0.0.1",
		MTAPort:  25,
	}

	TestSMTPD(&daemon, t)
}

func TestTLS(t *testing.T) {
	t.Skip("This test case is prepared for developer to run manually")

	from := mail.Address{Name: "", Address: "testcase@example.com"}
	to := mail.Address{Name: "", Address: "testcase@example.com"}
	headers := make(map[string]string)
	headers["From"] = from.String()
	headers["To"] = to.String()
	headers["Subject"] = "Example subject"

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + "Email body"

	smtpAddr := "example.com:25"
	host, _, _ := net.SplitHostPort(smtpAddr)
	tlsConf := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         host,
	}

	smtpClient, err := smtp.Dial(smtpAddr)
	if err != nil {
		t.Fatal(err)
	}
	if err := smtpClient.StartTLS(tlsConf); err != nil {
		t.Fatal(err)
	}
	if err = smtpClient.Mail(from.Address); err != nil {
		t.Fatal(err)
	}
	if err = smtpClient.Rcpt(to.Address); err != nil {
		t.Fatal(err)
	}
	w, err := smtpClient.Data()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write([]byte(message)); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	if err = smtpClient.Quit(); err != nil {
		t.Fatal(err)
	}
}
