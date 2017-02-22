package mailp

import (
	"regexp"
	"strings"
)

var RegexMailAddress = regexp.MustCompile(`[a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+@[a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+.[a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+`)

/*
Read an entire email and figure out subject, content type, and address to reply to, all in lower case.
All return values are in lower case.
*/
func GetMailProperties(mailContent string) (subject string, contentType string, replyTo string) {
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
			subject = strings.TrimSpace(strings.TrimPrefix(line, "subject:"))
		} else if strings.HasPrefix(line, "content-type:") && contentType == "" {
			contentType = strings.TrimSpace(strings.TrimPrefix(line, "content-type:"))
		}
	}
	return
}
