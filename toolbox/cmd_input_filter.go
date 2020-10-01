// filter package contains transformation functions that may be combined in order to enrich command input and result.
package toolbox

import (
	"crypto/subtle"
	"errors"
	"strings"
	"sync"

	"github.com/HouzuoGuo/laitos/lalog"
)

/*
CommandFilter applies transformations to any aspect of input command, such as its content or timeout. The transform
function returns transformed command instead of modifying the command in-place.
*/
type CommandFilter interface {
	Transform(Command) (Command, error)
}

// TOTPWithCommand contains a TOTP (6+6=12 digits) and a toolbox command that is authenticated to execute with that TOTP.
type TOTPWithCommand struct {
	TOTP           string
	ToolboxCommand string
}

/*
lastTOTPCommandContent is a mapping between a password and the most recent toolbox command executed and authenticated via TOTP derived from that password.
The state helps to prevent an eavesdropper from reusing an intercepted good TOTP for a malicious command.
*/
var lastTOTPCommandContent = map[string]TOTPWithCommand{}
var lastTOTPCommandContentMutex = new(sync.Mutex)

/*
setLastTOTPCommand determiens whether the toolbox command may proceed to execute using the TOTP that has been proven valid.
The function returns true only if the TOTP is being used to authenticate the toolbox command for the first time, or, if
the identical toolbox command was executed quite recently using that TOTP.
If the TOTP is being used a second time to authenticate a different toolbox command, then the function will return false
without memorising the new toolbox command.
*/
func canExecuteCommandUsingTOTP(commandContent, totp, password string) bool {
	lastTOTPCommandContentMutex.Lock()
	defer lastTOTPCommandContentMutex.Unlock()
	if totpWithCommand, exists := lastTOTPCommandContent[password]; exists {
		if totpWithCommand.TOTP == totp {
			// It is OK to reuse the TOTP to authenticate the same toolbox command
			return totpWithCommand.ToolboxCommand == commandContent
		} else {
			// It is always OK to use a new TOTP authenticate any toolbox command
			lastTOTPCommandContent[password] = TOTPWithCommand{TOTP: totp, ToolboxCommand: commandContent}
			return true
		}
	} else {
		// The pasword is being used with TOTP for the first time, the toolbox command may proceed.
		lastTOTPCommandContent[password] = TOTPWithCommand{TOTP: totp, ToolboxCommand: commandContent}
		return true
	}
}

/*
PINAndShortcuts looks for:
- Any of the recognised password PIN found at the beginning of any of the input lines.
- Any of the recognised shortcut strings that matches the entirety of any of the input lines.
The filter's Transform function will return an error if nothing is found.
*/
type PINAndShortcuts struct {
	Passwords []string          `json:"Passwords"`
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
func getTOTP(password string) (ret map[string]bool) {
	ret = map[string]bool{}
	if password == "" {
		return
	}
	// Calculate TOTP using password PIN - list 1
	prev1, current1, next1, err := GetTwoFACodes(password)
	if err != nil {
		lalog.DefaultLogger.Info("getTOTP", "", err, "failed to calculate TOTP")
		return
	}
	// Reverse the password PIN
	reversed := []rune(password)
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
	if len(pin.Passwords) == 0 && len(pin.Shortcuts) == 0 {
		return Command{}, errors.New("PINAndShortcut must define security password(s), shortcut(s), or both.")
	}

	// Among the input lines, look for a shortcut match, password PIN match, or TOTP code match, and leave command alone for further processing.
	for _, line := range cmd.Lines() {
		line = strings.TrimSpace(line)
		// Look for shortcut match
		if pin.Shortcuts != nil {
			if shortcut, exists := pin.Shortcuts[line]; exists {
				ret := cmd
				// The shortcut's corresponding toolbox command is defined in the filter configuration
				ret.Content = shortcut
				return ret, nil
			}
		}
		// Look for a password PIN match
		for _, password := range pin.Passwords {
			// Calculate password-derived TOTP codes that can be used in place of password PIN
			if len(line) > len(password) && subtle.ConstantTimeCompare([]byte(line[:len(password)]), []byte(password)) == 1 {
				ret := cmd
				// Remove matched password from the input, leave the app command in-place.
				ret.Content = line[len(password):]
				return ret, nil
			}
			// Look for a TOTP code match. The code is made of two TOTP numbers with six digits each.
			if len(line) > 12 {
				totpCodes := getTOTP(password)
				totpInput := line[:12]
				if totpCodes[totpInput] {
					// Determine whether the valid TOTP may execute this toolbox command
					if !canExecuteCommandUsingTOTP(cmd.Content, totpInput, password) {
						return cmd, ErrTOTPAlreadyUsed
					}
					ret := cmd
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
