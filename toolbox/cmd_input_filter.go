// filter package contains transformation functions that may be combined in order to enrich command input and result.
package toolbox

import (
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/HouzuoGuo/laitos/lalog"
)

/*
CommandFilter applies transformations to any aspect of input command, such as its content or timeout. The transform
function returns transformed command instead of modifying the command in-place.
*/
type CommandFilter interface {
	Transform(Command) (Command, error)
}

/*
LastTOTP and LastTOTPCommandContent track the command executed and successfully authenticated via TOTP instead of password PIN entry.
A successfully authenticated TOTP may only be used to execute a single command - one or more times while the TOTP remains valid.
*/
var LastTOTP, LastTOTPCommandContent string

/*
Match prefix PIN (or pre-defined shortcuts) against lines among input command. Return the matched line trimmed
and without PIN prefix, or expanded shortcut if found.
To successfully expend shortcut, the shortcut must occupy the entire line, without extra prefix or suffix.
Return error if neither PIN nor pre-defined shortcuts matched any line of input command.
*/
type PINAndShortcuts struct {
	PIN       string            `json:"PIN"`
	Shortcuts map[string]string `json:"Shortcuts"`
}

var ErrPINAndShortcutNotFound = errors.New("invalid password PIN or shortcut")
var ErrTOTPAlreadyUsed = errors.New("the TOTP has already been used with a different command")

/*
getTOTP returns TOTP-based PINs that work as alternative to password PIN text input.
TOTP based PINs are calculated based on system clock, therefore, the function returns a set of acceptable numbers in 90 seconds interval
and user may use any of the numbers instead of password PIN, this helps to mask the real password PIN when executing app commands
over less-secure communication channels such as DNS queries and SMS text.

The TOTP number set is calculated this way:
1. List 1 = the previous, current, and upcoming TOTP 2FA codes based on password PIN string (each code is a string made of 6 digits).
2. List 2 = the previous, current, and upcoming TOTP 2FA codes based on the password PIN string reversed.
3. For each string from list 1, concatenate it with each string from list 2, and return the concatenation results in a set.
*/
func (pin *PINAndShortcuts) getTOTP() (ret map[string]bool) {
	ret = map[string]bool{}
	if pin.PIN == "" {
		return
	}
	// Calculate TOTP using password PIN - list 1
	prev1, current1, next1, err := GetTwoFACodes(pin.PIN)
	if err != nil {
		lalog.DefaultLogger.Info("getTOTP", "", err, "failed to calculate TOTP")
		return
	}
	// Reverse the password PIN
	reversed := []rune(pin.PIN)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	reversedStr := string(reversed)
	// Calculate TOTP using reversed password PIN - list 2
	prev2, current2, next2, err := GetTwoFACodes(reversedStr)
	if err != nil {
		lalog.DefaultLogger.Info("getTOTP", "", err, "failed to calculate TOTP")
		return
	}
	// Concatenate codes from the first list and second list
	for _, s1 := range []string{prev1, current1, next1} {
		for _, s2 := range []string{prev2, current2, next2} {
			if code := s1 + s2; len(code) != 12 {
				lalog.DefaultLogger.Info("getTOTP", "", nil, "wrong code length - %d", len(code))
				return
			}
			ret[s1+s2] = true
		}
	}
	return
}

func (pin *PINAndShortcuts) Transform(cmd Command) (Command, error) {
	if pin.PIN == "" && (pin.Shortcuts == nil || len(pin.Shortcuts) == 0) {
		return Command{}, errors.New("PINAndShortcut must use a password PIN, shortcut(s), or both.")
	}
	// Calculate password-derived TOTP codes that can be used in place of password PIN
	totpCodes := pin.getTOTP()

	// Among the input lines, look for a shortcut match, password PIN match, or TOTP code match, and leave command alone for further processing.
	for _, line := range cmd.Lines() {
		line = strings.TrimSpace(line)
		// Look for shortcut match
		if pin.Shortcuts != nil {
			if shortcut, exists := pin.Shortcuts[line]; exists {
				ret := cmd
				// Toolbox command comes from the shortcut's configuration
				ret.Content = shortcut
				return ret, nil
			}
		}
		// Look for a password PIN match
		if pin.PIN != "" {
			if len(line) > len(pin.PIN) && subtle.ConstantTimeCompare([]byte(line[:len(pin.PIN)]), []byte(pin.PIN)) == 1 {
				ret := cmd
				// Remove matched password from the input, leave the app command in-place.
				ret.Content = line[len(pin.PIN):]
				return ret, nil
			}
			// Look for a TOTP code match. The code is made of two TOTP numbers with six digits each.
			if len(line) > 12 {
				totpInput := line[:12]
				if totpCodes[totpInput] {
					// Prevent a TOTP from executing more than one commands in short succession
					if totpInput == LastTOTP && cmd.Content != LastTOTPCommandContent {
						return cmd, ErrTOTPAlreadyUsed
					}
					ret := cmd
					LastTOTP = totpInput
					LastTOTPCommandContent = cmd.Content
					// Remove matched TOTP from the input, leave the toolbox command in-place.
					ret.Content = line[12:]
					return ret, nil
				}
			}
		}
	}
	// Cannot match a shortcut, password, or TOTP code, the command must not be processed further.
	return cmd, ErrPINAndShortcutNotFound
}

// Translate character sequences to something different.
type TranslateSequences struct {
	Sequences [][]string `json:"Sequences"`
}

func (tr *TranslateSequences) Transform(cmd Command) (Command, error) {
	if tr.Sequences == nil {
		return cmd, nil
	}
	newContent := cmd.Content
	for _, tuple := range tr.Sequences {
		if len(tuple) != 2 {
			continue
		}
		newContent = strings.Replace(newContent, tuple[0], tuple[1], -1)
	}
	ret := cmd
	ret.Content = newContent
	return ret, nil
}
