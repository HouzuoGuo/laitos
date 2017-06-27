package feature

import (
	"errors"
	"github.com/HouzuoGuo/laitos/env"
	"os"
	"path"
	"runtime"
)

// Execute shell commands with a timeout limit.
type Shell struct {
	InterpreterPath string `json:"InterpreterPath"` // Path to *nix shell interpreter
}

func (sh *Shell) IsConfigured() bool {
	// Shell command execution is unavailable only on Windows
	return runtime.GOOS != "windows"
}

func (sh *Shell) SelfTest() error {
	if !sh.IsConfigured() {
		return errors.New("Incompatible OS")
	}
	// The timeout for testing shell is gracious enough to allow disk to spin up from sleep
	_, err := env.InvokeShell(sh.InterpreterPath, 10, "echo test")
	return err
}

func (sh *Shell) Initialise() error {
	if sh.InterpreterPath != "" {
		goto afterShell
	}
	// Find a shell interpreter with a preference to use bash
	for _, shellName := range []string{"bash", "dash", "tcsh", "ksh", "sh"} {
		for _, pathPrefix := range []string{"/bin", "/usr/bin", "/usr/local/bin"} {
			shellPath := path.Join(pathPrefix, shellName)
			if _, err := os.Stat(shellPath); err == nil {
				sh.InterpreterPath = shellPath
				if err := sh.SelfTest(); err == nil {
					goto afterShell
				}
				sh.InterpreterPath = ""
			}
		}
	}
afterShell:
	if sh.InterpreterPath == "" {
		return errors.New("Failed to find a working shell interpreter (bash/dash/tcsh/ksh/sh)")
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

	procOut, procErr := env.InvokeShell(sh.InterpreterPath, cmd.TimeoutSec, cmd.Content)
	return &Result{Error: procErr, Output: procOut}
}
