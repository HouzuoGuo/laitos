package smtpd

import (
	"fmt"
	"strings"
)

// SetHeader looks for the header in the mail message, and changes its value.
// If the header does not exist yet, it'll be added to the mail message.
func SetHeader(mail string, name string, value string) string {
	lines := strings.Split(mail, "\n")
	var out []string
	var found bool
	for _, line := range lines {
		// I *think* the standard actually dictates the header names are case insensitive.
		if strings.HasPrefix(strings.ToLower(line), strings.ToLower(name)+":") {
			found = true
			out = append(out, fmt.Sprintf("%s: %s", name, value))
		} else {
			out = append(out, line)
		}
	}
	if !found {
		out = append([]string{fmt.Sprintf("%s: %s", name, value)}, out...)
	}
	return strings.Join(out, "\n")
}

// GetHeader returns the value of the header.
func GetHeader(mail string, name string) string {
	lines := strings.Split(mail, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), strings.ToLower(name)+":") {
			return strings.TrimSpace(line[strings.Index(line, ":")+1:])
		}
	}
	return ""
}
