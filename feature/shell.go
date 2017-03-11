package feature

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path"
	"runtime"
	"time"
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
	_, err := sh.InvokeShell(10, "echo test")
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

// Invoke shell to run the content piece, return shell stdout+stderr combined and error if there is any.
func (sh *Shell) InvokeShell(timeoutSec int, content string) (out string, err error) {
	// Collect stdout and stderr all together in a single buffer
	var outBuf bytes.Buffer
	proc := exec.Command(sh.InterpreterPath, "-c", content)
	proc.Stdout = &outBuf
	proc.Stderr = &outBuf
	// Run the shell command in a separate routine in order to monitor for timeout
	procRunChan := make(chan error, 1)
	go func() {
		procRunChan <- proc.Run()
	}()
	select {
	case procErr := <-procRunChan:
		// Upon process completion, retrieve result.
		out = outBuf.String()
		err = procErr
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		// If timeout is reached yet the process still has not completed, kill it.
		out = outBuf.String()
		if proc.Process != nil {
			if err = proc.Process.Kill(); err == nil {
				err = ErrExecTimeout
			}
		}
	}
	return
}

func (sh *Shell) Execute(cmd Command) *Result {
	if errResult := cmd.Trim(); errResult != nil {
		return errResult
	}

	procOut, procErr := sh.InvokeShell(cmd.TimeoutSec, cmd.Content)
	return &Result{Error: procErr, Output: procOut}
}
