package smtpd

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// SetHeader changes the header value of the email payload.
func SetHeader(mail []byte, name string, value string, useCr bool) []byte {
	if len(mail) == 0 {
		return []byte{}
	}
	// Look no further than the first 16KB for the header.
	searchLimit := min(len(mail), 16*1024)
	titleCaseName := cases.Title(language.English).String(name)
	/*
		The Internet Message Format RFC does not say much about header's case sensitivity:
		https://tools.ietf.org/html/rfc5322#section-2.2
	*/
	var headerIndex int
	headerIndex = bytes.Index(mail[:searchLimit], []byte(strings.ToLower(name)+":"))
	if headerIndex == -1 {
		headerIndex = bytes.Index(mail[:searchLimit], []byte(strings.ToUpper(name)+":"))
		if headerIndex == -1 {
			headerIndex = bytes.Index(mail[:searchLimit], []byte(titleCaseName+":"))
		}
	}
	if headerIndex == -1 {
		// The specified header cannot be found.
		return mail
	}
	// Look for end (LF) of the header line
	lf := bytes.IndexByte(mail[headerIndex:searchLimit], 0x0a)
	if lf == -1 {
		// Header value too long - longer than 16KB?
		return mail
	}
	lf += headerIndex
	if useCr {
		return append(mail[:headerIndex], append([]byte(fmt.Sprintf(titleCaseName+": %s\r\n", value)), mail[lf+1:]...)...)
	} else {
		return append(mail[:headerIndex], append([]byte(fmt.Sprintf(titleCaseName+": %s\n", value)), mail[lf+1:]...)...)
	}
}

// GetHeader returns the value of the header.
func GetHeader(mail []byte, name string) string {
	if len(mail) == 0 {
		return ""
	}
	// Look no further than the first 16KB for the header.
	searchLimit := min(len(mail), 16*1024)
	titleCaseName := cases.Title(language.English).String(name)
	/*
		The Internet Message Format RFC does not say much about header's case sensitivity:
		https://tools.ietf.org/html/rfc5322#section-2.2
	*/
	var headerIndex int
	headerIndex = bytes.Index(mail[:searchLimit], []byte(strings.ToLower(name)+":"))
	if headerIndex == -1 {
		headerIndex = bytes.Index(mail[:searchLimit], []byte(strings.ToUpper(name)+":"))
		if headerIndex == -1 {
			headerIndex = bytes.Index(mail[:searchLimit], []byte(titleCaseName+":"))
		}
	}
	if headerIndex == -1 {
		// The specified header cannot be found.
		return ""
	}
	// Look for end (LF) of the header line
	lf := bytes.IndexByte(mail[headerIndex:searchLimit], 0x0a)
	if lf == -1 {
		// Header value too long - longer than 16KB?
		return ""
	}
	lf += headerIndex
	return strings.TrimSpace(string(mail[headerIndex+len(name)+2 : lf]))
}
