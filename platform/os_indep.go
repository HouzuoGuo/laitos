package platform

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

const (
	/*
		MaxExternalProgramOutputBytes is the maximum number of bytes (combined stdout and stderr) to keep for an
		external program for caller to retrieve.
	*/
	MaxExternalProgramOutputBytes = 1024 * 1024
)

var (
	// logger is used by some of the OS platform specific actions that affect laitos process globally.
	logger = &lalog.Logger{ComponentName: "platform", ComponentID: []lalog.LoggerIDField{{Key: "PID", Value: os.Getpid()}}}
)

type (
	// ExternalProcessStarter is the function signature for starting an external program.
	ExternalProcessStarter func([]string, int, io.WriteCloser, io.WriteCloser, chan<- error, <-chan struct{}, string, ...string) error
)

// InvokeShell starts the shell interpreter and passes the script content to "-c" flag.
// Nearly all shell interpreters across Linux and Windows accept the "-c" convention.
// Return stdout+stderr combined, the maximum size is capped to MaxExternalProgramOutputBytes.
func InvokeShell(timeoutSec int, interpreter string, content string) (out string, err error) {
	return InvokeProgram(nil, timeoutSec, interpreter, "-c", content)
}

// StartProgram starts an external process, with optionally added environment variables and timeout monitor.
// The function waits for the process to terminate, and then returns the error at termination (e.g. abnormal exit codde) if any.
func StartProgram(envVars []string, timeoutSec int, stdout, stderr io.WriteCloser, start chan<- error, terminate <-chan struct{}, program string, args ...string) error {
	if timeoutSec < 1 {
		return errors.New("invalid time limit")
	}
	defer func() {
		_ = stdout.Close()
		_ = stderr.Close()
	}()
	// The external process uses my environment variables mixed with hard-coded PATH and custom environment.
	// For duplicated keys, the last value of the key becomes effective.
	defaultOSEnv := os.Environ()
	combinedEnv := make([]string, 0, 1+len(defaultOSEnv))
	combinedEnv = append(combinedEnv, defaultOSEnv...)
	combinedEnv = append(combinedEnv, "PATH="+CommonPATH)
	if envVars != nil {
		combinedEnv = append(combinedEnv, envVars...)
	}
	// Supervise the process execution, put a time box around it.
	minuteTicker := time.NewTicker(1 * time.Minute)
	defer minuteTicker.Stop()
	unixSecAtStart := time.Now().Unix()
	timeLimitExceeded := time.After(time.Duration(timeoutSec) * time.Second)
	processExitChan := make(chan error, 1)
	absPath, err := filepath.Abs(program)
	if err != nil {
		return fmt.Errorf("failed to determine abs path of the program %q: %w", program, err)
	}
	var process *os.Process
	if localAppData := os.Getenv("LOCALAPPDATA"); len(localAppData) > 0 && strings.Contains(program, localAppData) {
		// The Windows execution path. os.Exec is incompatible with Windows.
		logger.Info(program, nil, "using os.StartProcess workaround to execute the program and will be unable to read program output")
		// StartProcess does not automatically prepend args with the executable path.
		args = append([]string{absPath}, args...)
		process, err = os.StartProcess(program, args, &os.ProcAttr{Env: envVars, Files: []*os.File{nil, nil, nil}})
		if err != nil {
			return fmt.Errorf("failed to execute program %q: %v", program, err)
		}
		go func() {
			status, exitErr := process.Wait()
			logger.Info(program, exitErr, "process %d exited with status %d after %d seconds", process.Pid, int64(status.ExitCode()), time.Now().Unix()-unixSecAtStart)
			processExitChan <- exitErr
		}()
	} else {
		// The Unix/Linux execution path.
		proc := exec.Command(program, args...)
		proc.Env = combinedEnv
		proc.Stdout = stdout
		proc.Stderr = stderr
		proc.SysProcAttr = extProcAttr
		startErr := proc.Start()
		if startErr != nil {
			start <- startErr
			return fmt.Errorf("failed to execute program %q: %v", program, err)
		}
		close(start)
		process = proc.Process
		go func() {
			exitErr := proc.Wait()
			if exitErr == nil {
				logger.Info(program, exitErr, "process %d exited normally after %d seconds", proc.Process.Pid, time.Now().Unix()-unixSecAtStart)
			} else {
				logger.Info(program, exitErr, "process %d exited abnormally after %d seconds", proc.Process.Pid, time.Now().Unix()-unixSecAtStart)
			}
			processExitChan <- exitErr
		}()
	}
	for {
		// Monitor long-duration process, time-out condition, and regular process exit.
		select {
		case <-minuteTicker.C:
			// If the the process may 10 minutes or longer to run, then start logging how much time the process has left every minute.
			if timeoutSec >= 10*60 {
				spentMinutes := (time.Now().Unix() - unixSecAtStart) / 60
				timeoutRemainingMinutes := (timeoutSec - int(time.Now().Unix()-unixSecAtStart)) / 60
				if process != nil {
					logger.Info(program, nil, "process %d has been running for %d minutes and will time out in %d minutes",
						process.Pid, spentMinutes, timeoutRemainingMinutes)
				}
			}
		case <-timeLimitExceeded:
			// Forcibly kill the process upon exceeding time limit
			if process != nil {
				logger.Warning(program, nil, "killing process %d due to time limit (%d seconds)", process.Pid, timeoutSec)
				if !KillProcess(process) {
					logger.Warning(program, nil, "failed to kill PID %d after time limit exceeded", process.Pid)
				}
			}
		case <-terminate:
			if process != nil {
				logger.Info(program, nil, "killing process %d by request", process.Pid)
				if !KillProcess(process) {
					logger.Warning(program, nil, "failed to kill PID %d", process.Pid)
				}
			}
		case exitErr := <-processExitChan:
			return exitErr
		}
	}
}

// InvokeProgram starts an external executable with optional added environment variables,
// and kills it before reacing the maximum execution timeout to prevent a runaway.
// It returns stdout+stderr combined, the maximum size is capped to MaxExternalProgramOutputBytes.
func InvokeProgram(envVars []string, timeoutSec int, program string, args ...string) (string, error) {
	outBuf := lalog.NewByteLogWriter(io.Discard, MaxExternalProgramOutputBytes)
	err := StartProgram(envVars, timeoutSec, outBuf, outBuf, make(chan<- error), make(<-chan struct{}), program, args...)
	return string(outBuf.Retrieve(false)), err
}
