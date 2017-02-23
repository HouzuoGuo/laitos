package bridge

import (
	"bytes"
	"fmt"
	"github.com/HouzuoGuo/websh/feature"
	"log"
	"net/smtp"
	"regexp"
	"strings"
	"unicode"
)

var RegexConsecutiveSpaces = regexp.MustCompile("[[:space:]]+") // match consecutive space characters

/*
Provide transformation feature for command result. Unlike command bridges that manipulates and return new command records,
result bridges directly manipulate the result record.
*/
type ResultBridge interface {
	Transform(*feature.Result) error // Operate on the command result. Return an error if no further transformation shall be done.
}

// Call "ResetCombinedText()" function on the command result, so that the text will be available for further manipulation.
type ResetCombinedText struct {
}

func (_ *ResetCombinedText) Transform(result *feature.Result) error {
	// This looks dumb
	result.ResetCombinedText()
	return nil
}

/*
Lint combined text string in the following order (each step is turned on by respective attribute)
1. Trim all leading and trailing spaces from lines.
2. Compress all lines into a single line, joint by a semicolon.
3. Retain only printable & visible, 7-bit ASCII characters.
4. Compress consecutive spaces into single space - this will also cause all lines to squeeze.
5. Remove a number of leading character.
6. Remove excessive characters at end of the string.
*/
type LintCombinedText struct {
	TrimSpaces              bool
	CompressToSingleLine    bool
	KeepVisible7BitCharOnly bool
	CompressSpaces          bool
	BeginPosition           int
	MaxLength               int
}

func (lint *LintCombinedText) Transform(result *feature.Result) error {
	ret := result.CombinedOutput
	// Trim
	if lint.TrimSpaces {
		var out bytes.Buffer
		for _, line := range strings.Split(ret, "\n") {
			out.WriteString(strings.TrimSpace(line))
			out.WriteRune('\n')
		}
		ret = out.String()
	}
	// Compress lines
	if lint.CompressToSingleLine {
		ret = strings.Replace(ret, "\n", ";", -1)
	}
	// Retain printable chars
	if lint.KeepVisible7BitCharOnly {
		var out bytes.Buffer
		for _, r := range ret {
			if r < 128 && unicode.IsPrint(r) {
				out.WriteRune(r)
			}
		}
		ret = out.String()
	}
	// Compress consecutive spaces
	if lint.CompressSpaces {
		ret = RegexConsecutiveSpaces.ReplaceAllString(ret, " ")
	}
	// Cut leading characters
	if lint.BeginPosition > 0 {
		if len(ret) > lint.BeginPosition {
			ret = ret[lint.BeginPosition:]
		} else {
			ret = ""
		}
	}
	// Cut trailing characters
	if lint.MaxLength > 0 {
		if len(ret) > lint.MaxLength {
			ret = ret[0:lint.MaxLength]
		}
	}
	result.CombinedOutput = ret
	return nil
}

// Send email notification for command result.
type NotifyViaEmail struct {
	Recipients     []string // Email recipient addresses
	MailFrom       string   // FROM address of the outgoing emails
	MTAAddressPort string   // Address and port number of mail transportation agent
}

// Return true only if all mail parameters are present.
func (email *NotifyViaEmail) IsConfigured() bool {
	return email.Recipients != nil && len(email.Recipients) > 0 && email.MailFrom != "" && email.MTAAddressPort != ""
}

const MailNotificationGreeting = "websh notification" // These terms appear in the subject of notification emails

var FormatNotificationEmail = "Subject: " + MailNotificationGreeting + " - %s\r\n\r\n%s" // Subject and body format of notification emails

func (email *NotifyViaEmail) Transform(result *feature.Result) error {
	if email.IsConfigured() {
		go func() {
			msg := fmt.Sprintf(FormatNotificationEmail, result.Command, result.CombinedOutput)
			if err := smtp.SendMail(email.MTAAddressPort, nil, email.MailFrom, email.Recipients, []byte(msg)); err != nil {
				log.Printf("NotifyViaEmail: failed to send email for command \"%s\" - %v", result.Command.Content, err)
			}
		}()
	}
	return nil
}

// If there is no graph character among the combined output, replace it by "EMPTY OUTPUT".
type SayEmptyOutput struct {
}

var RegexGraphChar = regexp.MustCompile("[[:graph:]]") // Match any visible character
const EmptyOutputText = "EMPTY OUTPUT"                 // Text to substitute empty combined output with (SayEmptyOutput)

func (empty *SayEmptyOutput) Transform(result *feature.Result) error {
	if !RegexGraphChar.MatchString(result.CombinedOutput) {
		result.CombinedOutput = EmptyOutputText
	}
	return nil
}
