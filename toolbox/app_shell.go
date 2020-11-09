package toolbox

import (
	"context"
	"errors"
	"fmt"

	"github.com/HouzuoGuo/laitos/platform"
)

// Execute shell commands with a timeout limit.
type Shell struct {
	InterpreterPath string `json:"InterpreterPath"` // Path to *nix shell interpreter
}

func (sh *Shell) IsConfigured() bool {
	// There is always a shell executor available, no matter what the host system is.
	return true
}

func (sh *Shell) SelfTest() error {
	if !sh.IsConfigured() {
		return errors.New("Shell.SelfTest: OS is not compatible")
	}
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
	procOut, procErr := platform.InvokeShell(cmd.TimeoutSec, sh.InterpreterPath, cmd.Content)
	return &Result{Error: procErr, Output: procOut}
}
