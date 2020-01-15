package platform

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

// GetRootDiskUsageKB returns used and total space of the file system mounted on /. Returns 0 if they cannot be determined.
func GetRootDiskUsageKB() (usedKB, freeKB, totalKB int) {
	return 0, 0, 0
}

/*
InvokeProgram launches an external program with time constraints. The external program inherits laitos' environment
mixed with additional input environment variables. The additional variables take precedence over inherited ones.
Once the external program is launched, its scheduling priority is lowered to "below normal", as a safety measure,
because Windows is pretty bad keeping up when system is busy.
Returns stdout+stderr output combined, and error if there is any. The maximum amount of output returned is capped to
MaxExternalProgramOutputBytes.
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
	outBuf := lalog.NewByteLogWriter(ioutil.Discard, MaxExternalProgramOutputBytes)
	proc := exec.Command(program, args...)
	proc.Env = combinedEnv
	proc.Stdout = outBuf
	proc.Stderr = outBuf
	// Start external process
	unixSecAtStart := time.Now().Unix()
	timeLimitExceeded := time.After(time.Duration(timeoutSec) * time.Second)
	if err = proc.Start(); err != nil {
		return
	}
	// Wait for the process to finish
	processExitChan := make(chan error, 1)
	go func() {
		/*
			Lower the external process priority to "below normal" (magic priority number 16384). If an error occurs, it
			usually means the external process is very short lived. There is no need to log WMIC's error.
		*/
		_, _ = exec.Command(`C:\WINDOWS\System32\Wbem\WMIC.exe`, "process", "where", "ProcessID="+strconv.Itoa(proc.Process.Pid), "call", "SetPriority", "16384").CombinedOutput()
		exitErr := proc.Wait()
		if exitErr == nil {
			logger.Info("InvokeProgram", program, nil, "process exited normally after %d seconds", time.Now().Unix()-unixSecAtStart)
		} else {
			logger.Info("InvokeProgram", program, nil, "process exited after %d seconds due to: %v", time.Now().Unix()-unixSecAtStart, exitErr)
		}
		processExitChan <- exitErr
	}()
	minuteTicker := time.NewTicker(1 * time.Minute)
processMonitorLoop:
	for {
		// Monitor long-duration process, time-out condition, and regular process exit.
		select {
		case <-minuteTicker.C:
			// If the the process may 10 minutes or longer to run, then start logging how much time the process has left every minute.
			if timeoutSec >= 10*60 {
				spentMinutes := (time.Now().Unix() - unixSecAtStart) / 60
				timeoutRemainingMinutes := (timeoutSec - int(time.Now().Unix()-unixSecAtStart)) / 60
				logger.Info("InvokeProgram", program, nil, "external process %d has been running for %d minutes and will time out in %d minutes",
					proc.Process.Pid, spentMinutes, timeoutRemainingMinutes)
			}
		case <-timeLimitExceeded:
			// Forcibly kill the process upon exceeding time limit
			logger.Warning("InvokeProgram", program, nil, "killing the program due to time limit (%d seconds)", timeoutSec)
			if proc.Process != nil && !KillProcess(proc.Process) {
				logger.Warning("InvokeProgram", program, nil, "failed to kill after time limit exceeded")
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
	out = string(outBuf.Retrieve(false))
	return
}

// KillProcess kills the process or the group of processes associated with it.
func KillProcess(proc *os.Process) (success bool) {
	if proc == nil {
		return true
	}
	// Usage of taskkill.exe is explained in: https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/taskkill
	err := exec.Command(`C:\Windows\system32\taskkill.exe`, "/f", "/t", "/pid", strconv.Itoa(proc.Pid)).Run()
	if err == nil {
		success = true
	}
	if proc.Kill() == nil {
		success = true
	}
	_, _ = proc.Wait()
	return
}

// LockMemory locks program memory to prevent swapping, protecting sensitive user data.
func LockMemory() {
	logger.Warning("LockMemory", "", nil, "memory locking is not supported on Windows, your private information may leak onto disk.")
}
