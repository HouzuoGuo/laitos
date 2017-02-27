package email

import (
	"fmt"
	"net/smtp"
	"strings"
)

const OutgoingMailSubjectKeyword = "websh" // Outgoing emails are encouraged to carry this string in their subject

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

// Deliver mail to all recipients.
func (mailer *Mailer) Send(subject string, textBody string, recipients ...string) error {
	var auth smtp.Auth
	if mailer.AuthUsername != "" {
		auth = smtp.PlainAuth("", mailer.AuthUsername, mailer.AuthPassword, mailer.MTAHost)
	}
	// Construct appropriate mail headers
	mailBody := fmt.Sprintf("MIME-Version: 1.0\r\nContent-type: text/plain; charset=UTF-8\r\nFrom: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		mailer.MailFrom, strings.Join(recipients, ", "), subject, textBody)
	return smtp.SendMail(fmt.Sprintf("%s:%d", mailer.MTAHost, mailer.MTAPort), auth, mailer.MailFrom, recipients, []byte(mailBody))
}
