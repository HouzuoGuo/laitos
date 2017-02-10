package feature

import (
	"bytes"
	"errors"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

// Implement Result interface for shell command execution.
type ShellResult struct {
	Error     error
	RawOutput []byte
}

func (shResult *ShellResult) Err() error {
	return shResult.Error
}

func (shResult *ShellResult) ErrText() string {
	if shResult.Error == nil {
		return ""
	}
	return shResult.Error.Error()
}

func (shResult *ShellResult) OutText() string {
	if shResult.RawOutput == nil {
		return ""
	}
	return string(shResult.RawOutput)
}

func (shResult *ShellResult) CombinedText() (ret string) {
	errText := shResult.ErrText()
	outText := shResult.OutText()
	if errText != "" {
		ret = errText
		if outText != "" {
			ret += COMBINED_TEXT_SEP
		}
	}
	ret += outText
	return
}

// Execute shell commands with a timeout limit.
type Shell struct {
	InterpreterPath string // Path to shell interpreter
}

func (sh *Shell) InitAndTest() error {
	// Find a shell interpreter with a strong preference to use bash
	for _, shellName := range []string{"bash", "dash", "tcsh", "ksh", "sh"} {
		for _, pathPrefix := range []string{"/bin", "/usr/bin", "/usr/local/bin"} {
			shellPath := path.Join(pathPrefix, shellName)
			if _, err := os.Stat(shellPath); err == nil {
				sh.InterpreterPath = shellPath
				log.Printf("Shell.InitAndTest: will use shell interpreter at %s", shellPath)
				goto afterShell
			}
		}
	}
afterShell:
	if sh.InterpreterPath == "" {
		return errors.New("Failed to find a shell interpreter (bash/dash/tcsh/ksh/sh)")
	}
	return nil
}

func (sh *Shell) TriggerPrefix() string {
	return ".s"
}

func (sh *Shell) Execute(timeoutSec int, cmd string) (ret Result) {
	cmd = strings.TrimSpace(cmd)
	log.Printf("Shell.Execute: will run command - %s", cmd)
	if len(cmd) == 0 {
		return &ShellResult{Error: ErrEmptyCommand}
	}
	// Collect stdout and stderr all together in a single buffer
	var outBuf bytes.Buffer
	proc := exec.Command("/bin/bash", "-c", cmd)
	proc.Stdout = &outBuf
	proc.Stderr = &outBuf
	// Run the shell command in a separate routine in order to monitor for timeout
	procRunChan := make(chan error, 1)
	go func() {
		procRunChan <- proc.Run()
	}()
	var resultOut []byte
	var resultErr error
	select {
	case procErr := <-procRunChan:
		// Upon process completion, retrieve result.
		resultOut = outBuf.Bytes()
		resultErr = procErr
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		// If timeout is reached yet the process still has not completed, kill it.
		resultOut = outBuf.Bytes()
		if proc.Process != nil {
			if resultErr = proc.Process.Kill(); resultErr == nil {
				resultErr = ErrExecTimeout
			}
		}
	}
	ret = &ShellResult{Error: resultErr, RawOutput: resultOut}
	log.Printf("Shell.Execute: command '%s' has completed with result - %s", cmd, ret.CombinedText())
	return
}
