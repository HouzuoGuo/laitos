package main

import (
	"fmt"
	"log"
	"net/smtp"
	"regexp"
)

const (
	mailMagicKeyword = "websh" // Magic keyword that appears in mail notifications to prevent recursion in mail processing
)

var mailAddressRegex = regexp.MustCompile(`[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+`) // Match a mail address in header
var mailNotificationFormat = "Subject: " + mailMagicKeyword + " - %s\r\n\r\n%s"             // Subject and body format of notification and reply mails

type Mailer struct {
	Recipients     []string // List of mail addresses that receive notification after each command
	MailFrom       string   // FROM address of the mail notifications
	MTAAddressPort string   // Address and port number of mail transportation agent for sending notifications
}

// Return true only if all mail parameters are present (hence notification mails will be sent).
func (mail *Mailer) IsEnabled() bool {
	return mail.MTAAddressPort != "" && mail.MailFrom != "" && mail.Recipients != nil && len(mail.Recipients) > 0
}

// If mail parameters are present, send a notification mail in background for the specified command and output.
func (mail *Mailer) SendNotification(command, output string) {
	if !mail.IsEnabled() {
		return
	}
	go func() {
		msg := fmt.Sprintf(mailNotificationFormat, command, output)
		if err := smtp.SendMail(mail.MTAAddressPort, nil, mail.MailFrom, mail.Recipients, []byte(msg)); err == nil {
			log.Printf("Mail notification has been sent to '%v' for '%s'", mail.Recipients, command)
		} else {
			log.Printf("Failed to send mail notification for '%s' - %v", command, err)
		}
	}()
}
