package frontend

import (
	"regexp"
	"strings"
)

const WEBSH_MAIL_SUBJECT_KEYWORD = "websh" // the keyword to appear in all outgoing mails sent by websh

var RegexMailAddress = regexp.MustCompile(`[a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+@[a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+.[a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+`)

type MailProcessorConfig struct {
	PIN string
}

type MailProcessor struct {
	Features *FeatureSet
	Config   MailProcessorConfig
}

/*
Read an entire email and figure out subject, content type, and address to reply to.
Return nothing if the email appears to be originated from websh itself.
All return values are in lower case.
*/
func (mailp *MailProcessor) GetMailProperties(mailContent string) (subject string, contentType string, replyTo string) {
	for _, line := range strings.Split(mailContent, "\n") {
		line = strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(line, "from:") && replyTo == "" {
			if address := RegexMailAddress.FindString(line); address != "" {
				replyTo = address
			}
		} else if strings.HasPrefix(line, "reply-to:") {
			// "reply-to" address is preferred over "from" address
			if address := RegexMailAddress.FindString(line); address != "" {
				replyTo = address
			}
		} else if strings.HasPrefix(line, "subject:") {
			if strings.Contains(line, strings.ToLower(WEBSH_MAIL_SUBJECT_KEYWORD)) {
				// Immediately return nothing to avoid potential recursion error
				return "", "", ""
			} else {
				subject = strings.TrimSpace(strings.TrimPrefix(line, "subject:"))
			}
		} else if strings.HasPrefix(line, "content-type:") && contentType == "" {
			contentType = strings.TrimSpace(strings.TrimPrefix(line, "content-type:"))
		}
	}
	return
}
