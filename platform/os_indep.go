package platform

import (
	"errors"
	"fmt"
	"io/ioutil"
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

/*
InvokeShell launches an external shell process with time constraints to run a piece of shell code. The code is fed into
shell command parameter "-c", which happens to be universally accepted by Unix shells and Windows Powershell.
Returns shell stdout+stderr output combined and error if there is any. The maximum acount of output is capped to
MaxExternalProgramOutputBytes.
*/
func InvokeShell(timeoutSec int, interpreter string, content string) (out string, err error) {
	return InvokeProgram(nil, timeoutSec, interpreter, "-c", content)
}

/*
InvokeProgram launches an external program with time constraints. The external program inherits laitos' environment
mixed with additional input environment variables. The additional variables take precedence over inherited ones.
Returns stdout+stderr output combined, and error if there is any. The maximum amount of output returned is capped to
MaxExternalProgramOutputBytes.
*/
func InvokeProgram(envVars []string, timeoutSec int, program string, args ...string) (string, error) {
	if timeoutSec < 1 {
		return "", errors.New("invalid time limit")
	}
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
	unixSecAtStart := time.Now().Unix()
	timeLimitExceeded := time.After(time.Duration(timeoutSec) * time.Second)
	processExitChan := make(chan error, 1)
	outBuf := lalog.NewByteLogWriter(ioutil.Discard, MaxExternalProgramOutputBytes)
	absPath, err := filepath.Abs(program)
	if err != nil {
		return "", fmt.Errorf("failed to determine abs path of the program %q: %w", program, err)
	}
	var process *os.Process
	if localAppData := os.Getenv("LOCALAPPDATA"); len(localAppData) > 0 && strings.Contains(program, localAppData) {
		// Workaround os.Exec's incompatibility with Windows.
		logger.Info(program, nil, "using os.StartProcess workaround to execute the program and will be unable to read program output")
		// StartProcess does not automatically prepend args with the executable path.
		args = append([]string{absPath}, args...)
		process, err = os.StartProcess(program, args, &os.ProcAttr{Env: envVars, Files: []*os.File{nil, nil, nil}})
		if err != nil {
			return "", fmt.Errorf("failed to execute program %q: %v", program, err)
		}
		go func() {
			status, exitErr := process.Wait()
			logger.Info(program, exitErr, "process %d exited with status %d after %d seconds", process.Pid, int64(status.ExitCode()), time.Now().Unix()-unixSecAtStart)
			processExitChan <- exitErr
		}()
	} else {
		// Collect stdout and stderr all together in a single buffer
		proc := exec.Command(program, args...)
		proc.Env = combinedEnv
		proc.Stdout = outBuf
		proc.Stderr = outBuf
		proc.SysProcAttr = extProcAttr
		// Start external process
		if err = proc.Start(); err != nil {
			return "", fmt.Errorf("failed to execute program %q: %v", program, err)
		}
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
processMonitorLoop:
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
			err = errors.New("time limit exceeded")
			minuteTicker.Stop()
			break processMonitorLoop
		case exitErr := <-processExitChan:
			// Normal or abnormal exit that is not a time out
			err = exitErr
			minuteTicker.Stop()
			break processMonitorLoop
		}
	}
	return string(outBuf.Retrieve(false)), err
}
