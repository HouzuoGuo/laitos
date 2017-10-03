package inet

import (
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

const (
	OutgoingMailSubjectKeyword = "laitos" // Outgoing emails are encouraged to carry this string in their subject
	SelfTestTimeoutSec         = 10       // Timeout seconds for contacting MTA
)

// Send emails via SMTP.
type Mailer struct {
	MailFrom     string `json:"MailFrom"`     // FROM address of the outgoing mails
	MTAHost      string `json:"MTAHost"`      // Server name or IP address of mail transportation agent
	MTAPort      int    `json:"MTAPort"`      // Port number of SMTP service on mail transportation agent
	AuthUsername string `json:"AuthUsername"` // (Optional) Username for plain authentication, if the SMTP server requires it.
	AuthPassword string `json:"AuthPassword"` // (Optional) Password for plain authentication, if the SMTP server requires it.
}

// Return true only if all mail parameters are present.
func (mailer *Mailer) IsConfigured() bool {
	return mailer.MailFrom != "" && mailer.MTAHost != "" && mailer.MTAPort != 0
}

// Deliver mail to all recipients. Block until mail is sent or an error has occurred.
func (mailer *Mailer) Send(subject string, textBody string, recipients ...string) error {
	if recipients == nil || len(recipients) == 0 {
		return fmt.Errorf("No recipient specified for mail \"%s\"", subject)
	}
	var auth smtp.Auth
	if mailer.AuthUsername != "" {
		auth = smtp.PlainAuth("", mailer.AuthUsername, mailer.AuthPassword, mailer.MTAHost)
	}
	// Construct appropriate mail headers
	mailBody := fmt.Sprintf("MIME-Version: 1.0\r\nContent-type: text/plain; charset=utf-8\r\nFrom: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		mailer.MailFrom, strings.Join(recipients, ", "), subject, textBody)
	return smtp.SendMail(fmt.Sprintf("%s:%d", mailer.MTAHost, mailer.MTAPort), auth, mailer.MailFrom, recipients, []byte(mailBody))
}

// Deliver unmodified mail body to all recipients. Block until mail is sent or an error has occurred.
func (mailer *Mailer) SendRaw(fromAddr string, rawMailBody []byte, recipients ...string) error {
	if recipients == nil || len(recipients) == 0 {
		return fmt.Errorf("No recipient specified for mail from \"%s\"", fromAddr)
	}
	var auth smtp.Auth
	if mailer.AuthUsername != "" {
		auth = smtp.PlainAuth("", mailer.AuthUsername, mailer.AuthPassword, mailer.MTAHost)
	}
	return smtp.SendMail(fmt.Sprintf("%s:%d", mailer.MTAHost, mailer.MTAPort), auth, fromAddr, recipients, rawMailBody)
}

// Try to contact MTA and see if connection is possible.
func (mailer *Mailer) SelfTest() error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", mailer.MTAHost, mailer.MTAPort), 10*time.Second)
	if err != nil {
		return fmt.Errorf("Mailer.SelfTest: connection test failed - %v", err)
	}
	conn.Close()
	return nil
}
