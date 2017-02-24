package bridge

import (
	"bytes"
	"github.com/HouzuoGuo/websh/email"
	"github.com/HouzuoGuo/websh/feature"
	"log"
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
	TrimSpaces              bool `json:"TrimSpaces"`
	CompressToSingleLine    bool `json:"CompressToSingleLine"`
	KeepVisible7BitCharOnly bool `json:"KeepVisible7BitCharOnly"`
	CompressSpaces          bool `json:"CompressSpaces"`
	BeginPosition           int  `json:"BeginPosition"`
	MaxLength               int  `json:"MaxLength"`
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
		// Remove the suffix \n
		out.Truncate(out.Len() - 1)
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
	Recipients []string // Email recipient addresses
	Mailer     *email.Mailer
}

// Return true only if all mail parameters are present.
func (email *NotifyViaEmail) IsConfigured() bool {
	return email.Recipients != nil && len(email.Recipients) > 0 && email.Mailer != nil && email.Mailer.IsConfigured()
}

const MailNotificationGreeting = "websh notification" // These terms appear in the subject of notification emails

func (email *NotifyViaEmail) Transform(result *feature.Result) error {
	if email.IsConfigured() {
		go func() {
			if err := email.Mailer.Send(MailNotificationGreeting+result.Command.Content, result.CombinedOutput, email.Recipients...); err != nil {
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
