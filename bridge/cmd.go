package bridge

import "github.com/HouzuoGuo/websh/feature"

// Expand shortcuts to full commands.
type CommandShortcuts struct {
	Shortcuts map[string]string
}

func (shortcut *CommandShortcuts) Transform(cmd feature.Command) feature.Command {
	return
}

// Translate certain character sequences to something else.
type CommandTranslator struct {
	Sequences map[string]string
}

func (trans *CommandTranslator) Transform(cmd feature.Command) feature.Command {
	return
}
