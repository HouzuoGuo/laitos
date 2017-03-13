// Apply transformation functions to input command or result string.
package bridge

import (
	"errors"
	"github.com/HouzuoGuo/laitos/feature"
	"strings"
)

// Provide transformation feature for input command.
type CommandBridge interface {
	Transform(feature.Command) (feature.Command, error)
}

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

var ErrPINAndShortcutNotFound = errors.New("Failed to match PIN/shortcut")

func (pin *PINAndShortcuts) Transform(cmd feature.Command) (feature.Command, error) {
	if pin.PIN == "" && (pin.Shortcuts == nil || len(pin.Shortcuts) == 0) {
		return feature.Command{}, errors.New("Both PIN and shortcuts are undefined")
	}
	for _, line := range cmd.Lines() {
		line = strings.TrimSpace(line)
		// Try to match shortcut, then return expanded shortcut alone.
		if pin.Shortcuts != nil {
			if shortcut, exists := pin.Shortcuts[line]; exists {
				ret := cmd
				ret.Content = shortcut
				return ret, nil
			}
		}
		// Try to match PIN prefix, then remove it from successfully matched line.
		if len(line) > len(pin.PIN) && line[0:len(pin.PIN)] == pin.PIN {
			ret := cmd
			ret.Content = line[len(pin.PIN):]
			return ret, nil
		}
	}
	// Nothing matched
	return cmd, ErrPINAndShortcutNotFound
}

// Translate character sequences to something different.
type TranslateSequences struct {
	Sequences [][]string `json:"Sequences"`
}

func (tr *TranslateSequences) Transform(cmd feature.Command) (feature.Command, error) {
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
