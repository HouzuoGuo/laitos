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
type MailClient struct {
	MailFrom     string `json:"MailFrom"`     // FROM address of the outgoing mails
	MTAHost      string `json:"MTAHost"`      // Server name or IP address of mail transportation agent
	MTAPort      int    `json:"MTAPort"`      // Port number of SMTP service on mail transportation agent
	AuthUsername string `json:"AuthUsername"` // (Optional) Username for plain authentication, if the SMTP server requires it.
	AuthPassword string `json:"AuthPassword"` // (Optional) Password for plain authentication, if the SMTP server requires it.
}

// Return true only if all mail parameters are present.
func (client *MailClient) IsConfigured() bool {
	return client.MailFrom != "" && client.MTAHost != "" && client.MTAPort != 0
}

// Deliver mail to all recipients. Block until mail is sent or an error has occurred.
func (client *MailClient) Send(subject string, textBody string, recipients ...string) error {
	if recipients == nil || len(recipients) == 0 {
		return fmt.Errorf("No recipient specified for mail \"%s\"", subject)
	}
	var auth smtp.Auth
	if client.AuthUsername != "" {
		auth = smtp.PlainAuth("", client.AuthUsername, client.AuthPassword, client.MTAHost)
	}
	// Construct appropriate mail headers
	mailBody := fmt.Sprintf("MIME-Version: 1.0\r\nContent-type: text/plain; charset=utf-8\r\nFrom: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		client.MailFrom, strings.Join(recipients, ", "), subject, textBody)
	return smtp.SendMail(fmt.Sprintf("%s:%d", client.MTAHost, client.MTAPort), auth, client.MailFrom, recipients, []byte(mailBody))
}

// Deliver unmodified mail body to all recipients. Block until mail is sent or an error has occurred.
func (client *MailClient) SendRaw(fromAddr string, rawMailBody []byte, recipients ...string) error {
	if recipients == nil || len(recipients) == 0 {
		return fmt.Errorf("No recipient specified for mail from \"%s\"", fromAddr)
	}
	var auth smtp.Auth
	if client.AuthUsername != "" {
		auth = smtp.PlainAuth("", client.AuthUsername, client.AuthPassword, client.MTAHost)
	}
	return smtp.SendMail(fmt.Sprintf("%s:%d", client.MTAHost, client.MTAPort), auth, fromAddr, recipients, rawMailBody)
}

// Try to contact MTA and see if connection is possible.
func (client *MailClient) SelfTest() error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", client.MTAHost, client.MTAPort), SelfTestTimeoutSec*time.Second)
	if err != nil {
		return fmt.Errorf("MailClient.SelfTest: connection test failed - %v", err)
	}
	conn.Close()
	return nil
}
