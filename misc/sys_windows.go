package misc

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// GetRootDiskUsageKB returns used and total space of the file system mounted on /. Returns 0 if they cannot be determined.
func GetRootDiskUsageKB() (usedKB, freeKB, totalKB int) {
	return 0, 0, 0
}

/*
InvokeProgram launches an external program with time constraints. The external program inherits laitos' environment
mixed with additional input environment variables. The additional variables take precedence over inherited ones.
Returns stdout+stderr output combined, and error if there is any.
*/
func InvokeProgram(envVars []string, timeoutSec int, program string, args ...string) (out string, err error) {
	if timeoutSec < 1 {
		return "", errors.New("invalid time limit")
	}
	// Make an environment variable array of common PATH, inherited values, and newly specified values.
	defaultOSEnv := os.Environ()
	combinedEnv := make([]string, 0, 1+len(defaultOSEnv))
	// Inherit environment variables from program environment
	combinedEnv = append(combinedEnv, defaultOSEnv...)
	if envVars != nil {
		combinedEnv = append(combinedEnv, envVars...)
	}
	// Collect stdout and stderr all together in a single buffer
	var outBuf bytes.Buffer
	proc := exec.Command(program, args...)
	proc.Env = combinedEnv
	proc.Stdout = &outBuf
	proc.Stderr = &outBuf
	// Monitor for time out
	var timedOut bool
	timeOutTimer := time.AfterFunc(time.Duration(timeoutSec)*time.Second, func() {
		timedOut = true
		if !KillProcess(proc.Process) {
			logger.Warning("InvokeProgram", program, nil, "failed to kill after time limit exceeded")
		}
	})
	err = proc.Run()
	timeOutTimer.Stop()
	if timedOut {
		err = errors.New("time limit exceeded")
	}
	out = outBuf.String()
	return
}

// KillProcess kills the process or the group of processes associated with it.
func KillProcess(proc *os.Process) (success bool) {
	err := exec.Command(`C:\Windows\system32\taskkill.exe`, "/F", "/T", "/PID", strconv.Itoa(proc.Pid)).Run()
	if err == nil {
		success = true
	}
	return
}

// LockMemory locks program memory to prevent swapping, protecting sensitive user data.
func LockMemory() {
	logger.Warning("LockMemory", "", nil, "memory locking is not supported on Windows, your private information may leak onto disk.")
}
