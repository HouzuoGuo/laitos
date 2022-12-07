package toolbox

import (
	"context"
	"errors"
	"fmt"

	"github.com/HouzuoGuo/laitos/platform"
)

var ErrRestrictedShell = errors.New("restricted shell refuses to run the command")

// SafeShellCommands is a set of simple shell commands that are deemed safe for
// execution even for the restricted shell.
var SafeShellCommands = map[string]bool{
	// These one-word commands must be neither expensive nor modify anything
	// on the host system.
	"arch": true, "arp": true, "blkid": true, "cal": true, "date": true,
	"dmesg": true, "dnsdomainname": true, "false": true, "free": true,
	"groups": true, "hostid": true, "hostname": true, "id": true,
	"ifconfig": true, "ipconfig": true, "iostat": true, "ipcs": true, "kbd_mode": true,
	"ls": true, "lsof": true, "lspci": true, "lsusb": true, "mpstat": true,
	"netstat": true, "nproc": true, "ps": true, "pstree": true, "pwd": true,
	"route": true, "stty": true, "tty": true, "uname": true, "uptime": true,
	"whoami": true,
}

// Shell is a toolbox command that executes a shell statement.
type Shell struct {
	// Unrestricted enables execution of all free-form shell statements without
	// limitation.
	// By default only the commands in SafeShellCommands set are permitted.
	Unrestricted bool `json:"Unrestricted"`
	// InterpreterPath is the executable of shell interpreter.
	// It defaults to the shell interpreter auto-discovered from the OS host.
	InterpreterPath string `json:"InterpreterPath"`
}

func (sh *Shell) IsConfigured() bool {
	// There is always a shell executor available, no matter what the host system is.
	return true
}

func (sh *Shell) SelfTest() error {
	// The timeout for testing shell is gracious enough to allow disk to spin up from sleep
	if _, err := platform.InvokeShell(platform.CommonOSCmdTimeoutSec, sh.InterpreterPath, "echo test"); err != nil {
		return fmt.Errorf("Shell.SelfTest: interpreter \"%s\" is not working - %v", sh.InterpreterPath, err)
	}
	return nil
}

func (sh *Shell) Initialise() error {
	if sh.InterpreterPath == "" {
		sh.InterpreterPath = platform.GetDefaultShellInterpreter()
	}
	if sh.InterpreterPath == "" {
		return errors.New("Shell.Initialise: failed to find a working shell interpreter")
	}
	return nil
}

func (sh *Shell) Trigger() Trigger {
	return ".s"
}

func (sh *Shell) Execute(ctx context.Context, cmd Command) *Result {
	if errResult := cmd.Trim(); errResult != nil {
		return errResult
	}
	if !sh.Unrestricted {
		if !SafeShellCommands[cmd.Content] {
			return &Result{Error: ErrRestrictedShell}
		}
	}
	procOut, procErr := platform.InvokeShell(cmd.TimeoutSec, sh.InterpreterPath, cmd.Content)
	return &Result{Error: procErr, Output: procOut}
}
