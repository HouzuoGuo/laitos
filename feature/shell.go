package feature

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

const (
	/*
		The ultimate restrictions are quite strict themselves, feel free to open an issue
		if you find them overly strict.
	*/
	SHELL_RAW_OUTPUT_LEN_MAX = 16 * 1024 // Only return up to 16KB of command output
	SHELL_COMBINED_TEXT_SEP  = "|"       // Separate error and command output in the combined output
	SHELL_TIMEOUT_SEC_MIN    = 2         // Always permit commands to run for less than 2 seconds
	SHELL_TIMEOUT_SEC_MAX    = 2 * 60    // Never permit command to run for more than 120 seconds
)

var (
	ErrShellCommandEmpty = errors.New("shell command is empty")
	ErrShellTimeout      = errors.New("shell command timed out")
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
			ret += SHELL_COMBINED_TEXT_SEP
		}
	}
	ret += outText
	return
}

// Execute shell commands with a timeout limit.
type Shell struct {
	InterpreterPath string // Path to shell interpreter
	TimeoutSec      int    // In-progress commands are killed after this number of seconds
}

func (sh *Shell) InitAndTest() error {
	// Timeout must be within ultimate limits
	if sh.TimeoutSec < SHELL_TIMEOUT_SEC_MIN || sh.TimeoutSec > SHELL_TIMEOUT_SEC_MAX {
		return fmt.Errorf("Shell's TimeoutSec should be in between %d and %d", SHELL_TIMEOUT_SEC_MIN, SHELL_TIMEOUT_SEC_MAX)
	}
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

func (sh *Shell) Execute(cmd string) (ret Result) {
	cmd = strings.TrimSpace(cmd)
	log.Printf("Shell.Execute: will run command - %s", cmd)
	if len(cmd) == 0 {
		return &ShellResult{Error: ErrShellCommandEmpty}
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
	case <-time.After(time.Duration(sh.TimeoutSec) * time.Second):
		// If timeout is reached yet the process still has not completed, kill it.
		resultOut = outBuf.Bytes()
		if proc.Process != nil {
			if resultErr = proc.Process.Kill(); resultErr == nil {
				resultErr = ErrShellTimeout
			}
		}
	}
	// Cut excessive output data from result
	if len(resultOut) > SHELL_RAW_OUTPUT_LEN_MAX {
		resultOut = resultOut[0:SHELL_RAW_OUTPUT_LEN_MAX]
	}
	ret = &ShellResult{Error: resultErr, RawOutput: resultOut}
	log.Printf("Shell.Execute: command '%s' has completed with result - %s", cmd, ret.CombinedText())
	return
}
