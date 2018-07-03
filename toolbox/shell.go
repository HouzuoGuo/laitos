package toolbox

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"os"
	"path/filepath"
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
	if _, err := misc.InvokeShell(misc.CommonOSCmdTimeoutSec, sh.InterpreterPath, "echo test"); err != nil {
		return fmt.Errorf("Shell.SelfTest: interpreter \"%s\" is not working - %v", sh.InterpreterPath, err)
	}
	return nil
}

func (sh *Shell) Initialise() error {
	if sh.InterpreterPath != "" {
		goto afterShell
	}
	if misc.HostIsWindows() {
		sh.InterpreterPath = "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe"
	} else {
		// Find a Unix-style shell interpreter with a preference to use bash
		for _, shellName := range []string{"bash", "dash", "zsh", "ksh", "ash", "tcsh", "csh", "sh"} {
			for _, pathPrefix := range []string{"/bin", "/usr/bin", "/usr/local/bin", "/opt/bin"} {
				shellPath := filepath.Join(pathPrefix, shellName)
				if _, err := os.Stat(shellPath); err == nil {
					sh.InterpreterPath = shellPath
					if err := sh.SelfTest(); err == nil {
						goto afterShell
					}
					sh.InterpreterPath = ""
				}
			}
		}
	}
afterShell:
	if sh.InterpreterPath == "" {
		return errors.New("Shell.Initialise: failed to find a working shell interpreter")
	}
	return nil
}

func (sh *Shell) Trigger() Trigger {
	return ".s"
}

func (sh *Shell) Execute(cmd Command) *Result {
	if errResult := cmd.Trim(); errResult != nil {
		return errResult
	}
	procOut, procErr := misc.InvokeShell(cmd.TimeoutSec, sh.InterpreterPath, cmd.Content)
	return &Result{Error: procErr, Output: procOut}
}
