package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/smtp"
	"strings"
)

// Similar to an ordinary command runner, this one can find commands from Email body.
type MailProcessor struct {
	CommandRunner
	Mysterious MysteriousClient
}

// Validate configuration, make sure they look good.
func (proc *MailProcessor) CheckConfig() error {
	if err := proc.CommandRunner.CheckConfig(); err != nil {
		return err
	} else if !proc.CommandRunner.Mailer.IsEnabled() {
		return errors.New("Please complete all mail parameters")
	}
	return nil
}

// Look for a reply address in the mail text (reply-to or from). Return empty string if such address is not found.
func GetMailProperties(mailContent string) (subject string, contentType string, replyTo string) {
	for _, line := range strings.Split(mailContent, "\n") {
		trimmed := strings.TrimSpace(line)
		trimmedUpper := strings.ToUpper(trimmed)
		if strings.HasPrefix(trimmedUpper, "FROM:") && replyTo == "" {
			if address := mailAddressRegex.FindString(trimmed); address != "" {
				replyTo = address
			}
		} else if strings.HasPrefix(trimmedUpper, "REPLY-TO:") {
			// Reply-to is preferred over From
			if address := mailAddressRegex.FindString(trimmed); address != "" {
				replyTo = address
			}
		} else if strings.HasPrefix(trimmedUpper, "SUBJECT:") {
			if strings.Contains(trimmedUpper, strings.ToUpper(mailMagicKeyword)) {
				// Avoid recurse on mails sent by websh itself so return early
				return trimmed, "", ""
			} else {
				subject = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(trimmed), "subject:"))
			}
		} else if strings.HasPrefix(trimmedUpper, "CONTENT-TYPE:") && contentType == "" {
			contentType = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(trimmed), "content-type:"))
		}
	}
	return
}

// Process an entire mail message and run the command within.
func (proc *MailProcessor) RunCommandFromMail(mailContent string) {
	subject, contentType, replyTo := GetMailProperties(mailContent)
	if replyTo == "" {
		log.Printf("Failed to find reply address of mail '%s'", subject)
		return
	}
	log.Printf("Processing mail '%s' (type %s, reply to %s)...", subject, contentType, replyTo)
	if proc.Mysterious.IsEnabled() && strings.HasSuffix(replyTo, proc.Mysterious.Addr1) {
		log.Printf("Will respond to mail '%s' in undocumented ways", subject)
		cmd, output := proc.RunCommandFromMailBody(subject, contentType, mailContent)
		if cmd == "" {
			return
		}
		if err := proc.Mysterious.InvokeAPI(output); err != nil {
			log.Printf("Undocumented error happened while processing mail '%s' - %v", subject, err)
		}
	} else {
		// Match PIN/preset message in the mail body, run the command and reply
		cmd, output := proc.RunCommandFromMailBody(subject, contentType, mailContent)
		if cmd == "" {
			return
		}
		msg := fmt.Sprintf(mailNotificationFormat, cmd, output)
		if err := smtp.SendMail(proc.Mailer.MTAAddressPort, nil, proc.Mailer.MailFrom, []string{replyTo}, []byte(msg)); err != nil {
			log.Printf("Failed to reply to '%s' ('%s') via mail - %v", subject, replyTo, err)
		}
	}
}

// Find and run command from multipart or plain text mail content.
func (proc *MailProcessor) RunCommandFromMailBody(subject, contentType, mailContent string) (cmd, output string) {
	cmd = proc.FindCommandFromMultipartMail(contentType, subject, mailContent)
	if cmd == "" {
		cmd = proc.FindCommandFromPlainTextMail(subject, mailContent)
	}
	if cmd != "" {
		output = proc.RunCommand(cmd, false)
	}
	return
}

// Look for PIN/preset message match in the mail text (no multipart). Return empty if no match
func (proc *MailProcessor) FindCommandFromPlainTextMail(subject, mailContent string) string {
	for _, line := range strings.Split(mailContent, "\n") {
		if cmd := proc.FindCommand(line); cmd != "" {
			log.Printf("Plain text mail '%s' contains a command to run", subject)
			return cmd
		}
	}
	log.Printf("Plain text mail '%s' does not contain a command to run", subject)
	return ""
}

// Look for command or preset message match in the text part of a mutlipart mail. Return empty if no match.
func (proc *MailProcessor) FindCommandFromMultipartMail(contentType, subject, mailContent string) string {
	mimeMail := &mail.Message{
		Header: map[string][]string{"Content-Type": {contentType}},
		Body:   strings.NewReader(mailContent),
	}
	mediaType, params, err := mime.ParseMediaType(mimeMail.Header.Get("Content-Type"))
	if err == nil {
		if strings.HasPrefix(mediaType, "multipart/") {
			mr := multipart.NewReader(mimeMail.Body, params["boundary"])
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Printf("Failed to open multipart mail '%s' - %v", subject, err)
					break
				}
				slurp, err := ioutil.ReadAll(p)
				if err != nil {
					log.Printf("Failed to read multipart mail '%s' - %v", subject, err)
					break
				}
				partContentType := p.Header.Get("Content-Type")
				if strings.Contains(partContentType, "text") {
					if cmd := proc.FindCommandFromPlainTextMail(subject, string(slurp)); cmd != "" {
						log.Printf("Multipart mail '%s' contains a command to run", subject)
						return cmd
					}
				}
			}
		}
	}
	log.Printf("Multipart mail '%s' does not contain a command to run", subject)
	return ""
}
