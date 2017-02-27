package email

import (
	"fmt"
	"net/smtp"
)

// Send emails via SMTP.
type Mailer struct {
	MailFrom       string `json:"MailFrom"`       // FROM address of the outgoing mails
	MTAAddressPort string `json:"MTAAddressPort"` // Address and port number of mail transportation agent
}

// Return true only if all mail parameters are present.
func (mailer *Mailer) IsConfigured() bool {
	return mailer.MailFrom != "" && mailer.MTAAddressPort != ""
}

// Deliver mail to all recipients.
func (mailer *Mailer) Send(subject string, textBody string, recipients ...string) error {
	mailBody := fmt.Sprintf("Subject: %s\r\n\r\n%s", subject, textBody)
	return smtp.SendMail(mailer.MTAAddressPort, nil, mailer.MailFrom, recipients, []byte(mailBody))
}
