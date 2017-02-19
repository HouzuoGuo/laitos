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
	InterpreterPath string // Path to shell interpreter
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
	ret := sh.Execute(Command{TimeoutSec: 10, Content: "echo test"})
	return ret.Error
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

func (sh *Shell) TriggerPrefix() string {
	return ".s"
}

func (sh *Shell) Execute(cmd Command) (ret *Result) {
	LogBeforeExecute(cmd)
	defer func() {
		LogAfterExecute(cmd, ret)
	}()
	if errResult := cmd.Trim(); errResult != nil {
		ret = errResult
		return
	}

	// Collect stdout and stderr all together in a single buffer
	var outBuf bytes.Buffer
	proc := exec.Command("/bin/bash", "-c", cmd.Content)
	proc.Stdout = &outBuf
	proc.Stderr = &outBuf
	// Run the shell command in a separate routine in order to monitor for timeout
	procRunChan := make(chan error, 1)
	go func() {
		procRunChan <- proc.Run()
	}()
	var resultOut string
	var resultErr error
	select {
	case procErr := <-procRunChan:
		// Upon process completion, retrieve result.
		resultOut = outBuf.String()
		resultErr = procErr
	case <-time.After(time.Duration(cmd.TimeoutSec) * time.Second):
		// If timeout is reached yet the process still has not completed, kill it.
		resultOut = outBuf.String()
		if proc.Process != nil {
			if resultErr = proc.Process.Kill(); resultErr == nil {
				resultErr = ErrExecTimeout
			}
		}
	}
	ret = &Result{Error: resultErr, Output: resultOut}
	return
}
