package bridge

import (
	"bytes"
	"regexp"
	"strings"
	"unicode"
)

var RegexConsecutiveSpaces = regexp.MustCompile("[[:space:]]+")

// Provide transformation feature for command result.
type ResultBridge interface {
	Transform(string) (string, error)
}

/*
Lint string in the following order (each step is turned on by respective attribute)
1. Trim all leading and trailing spaces from lines.
2. Compress all lines into a single line, joint by a semicolon.
3. Retain only printable & visible, 7-bit ASCII characters.
4. Compress consecutive spaces into single space - this will also cause all lines to squeeze.
5. Remove a number of leading character.
6. Remove excessive characters at end of the string.
*/
type StringLint struct {
	TrimSpaces              bool
	CompressToSingleLine    bool
	KeepVisible7BitCharOnly bool
	CompressSpaces          bool
	BeginPosition           int
	MaxLength               int
}

func (lint *StringLint) Transform(in string) (string, error) {
	ret := in
	// Trim
	if lint.TrimSpaces {
		var out bytes.Buffer
		for _, line := range strings.Split(in, "\n") {
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
	return ret, nil
}
