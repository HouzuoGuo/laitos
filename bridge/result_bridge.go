package bridge

import (
	"bytes"
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/global"
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
type LintText struct {
	TrimSpaces              bool `json:"TrimSpaces"`
	CompressToSingleLine    bool `json:"CompressToSingleLine"`
	KeepVisible7BitCharOnly bool `json:"KeepVisible7BitCharOnly"`
	CompressSpaces          bool `json:"CompressSpaces"`
	BeginPosition           int  `json:"BeginPosition"`
	MaxLength               int  `json:"MaxLength"`
}

func (lint *LintText) Transform(result *feature.Result) error {
	ret := result.CombinedOutput
	// Trim
	if lint.TrimSpaces {
		var out bytes.Buffer
		for _, line := range strings.Split(ret, "\n") {
			out.WriteString(strings.TrimSpace(line))
			out.WriteRune('\n')
		}
		ret = strings.TrimSpace(out.String())
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
	Recipients []string      `json:"Recipients"` // Email recipient addresses
	Mailer     email.Mailer  `json:"-"`          // MTA that delivers outgoing notification email
	Logger     global.Logger `json:"-"`          // Logger
}

// Return true only if all mail parameters are present.
func (notify *NotifyViaEmail) IsConfigured() bool {
	return notify.Recipients != nil && len(notify.Recipients) > 0 && notify.Mailer.IsConfigured()
}

func (notify *NotifyViaEmail) Transform(result *feature.Result) error {
	if notify.IsConfigured() && result.Error != ErrPINAndShortcutNotFound {
		go func() {
			subject := email.OutgoingMailSubjectKeyword + "-notify-" + result.Command.Content
			if err := notify.Mailer.Send(subject, result.CombinedOutput, notify.Recipients...); err != nil {
				notify.Logger.Warningf("Transform", "NotifyViaEmail", err, "failed to send notification for command \"%s\"", result.Command.Content)
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
