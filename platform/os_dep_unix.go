// +build darwin linux

package platform

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

// GetRootDiskUsageKB returns used and total space of the file system mounted on /. Returns 0 if they cannot be determined.
func GetRootDiskUsageKB() (usedKB, freeKB, totalKB int) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs("/", &fs)
	if err != nil {
		return
	}
	totalKB = int(int64(fs.Blocks) * int64(fs.Bsize) / 1024)
	freeKB = int(int64(fs.Bfree) * int64(fs.Bsize) / 1024)
	usedKB = totalKB - freeKB
	return
}

/*
InvokeProgram launches an external program with time constraints. The external program inherits laitos' environment
mixed with additional input environment variables. The additional variables take precedence over inherited ones.
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
	/*
		Put common PATH values into the mix. Since go 1.9, when environment variables contain duplicated keys, only
		the last value of duplicated key is effective. This behaviour enables caller to override PATH if deemed
		necessary.
	*/
	combinedEnv = append(combinedEnv, "PATH="+CommonPATH)
	if envVars != nil {
		combinedEnv = append(combinedEnv, envVars...)
	}
	// Collect stdout and stderr all together in a single buffer
	outBuf := lalog.NewByteLogWriter(io.Discard, MaxExternalProgramOutputBytes)
	proc := exec.Command(program, args...)
	proc.Env = combinedEnv
	proc.Stdout = outBuf
	proc.Stderr = outBuf
	// Use process group so that child processes are also killed upon time out, Windows does not require this.
	proc.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Start external process
	unixSecAtStart := time.Now().Unix()
	timeLimitExceeded := time.After(time.Duration(timeoutSec) * time.Second)
	if err = proc.Start(); err != nil {
		return
	}
	// Wait for the process to finish
	processExitChan := make(chan error, 1)
	go func() {
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

// KillProcess kills the process and its child processes. The function gives the processes a second to clean up after themselves.
func KillProcess(proc *os.Process) (success bool) {
	if proc == nil {
		return true
	}
	// Send SIGTERM to the process group (if any) and the process itself
	if killErr := syscall.Kill(-proc.Pid, syscall.SIGTERM); killErr == nil {
		success = true
	}
	if killErr := syscall.Kill(proc.Pid, syscall.SIGTERM); killErr == nil {
		success = true
	}
	// Wait a second for the process to clean up after itself, then force their termination.
	time.Sleep(1 * time.Second)
	if killErr := syscall.Kill(-proc.Pid, syscall.SIGKILL); killErr == nil {
		success = true
	}
	if killErr := syscall.Kill(proc.Pid, syscall.SIGKILL); killErr == nil {
		success = true
	}
	// Use the built-in kill implementation as the last resort
	if proc.Kill() == nil {
		success = true
	}
	/*
		A killed process remains in process table, laitos as the parent process must retrieve
		the exit status, or the killed process will become a zombie.
	*/
	_, _ = proc.Wait()
	_ = proc.Release()
	return
}

// LockMemory locks program memory to prevent swapping, protecting sensitive user data.
func LockMemory() {
	// Lock all program memory into main memory to prevent sensitive data from leaking into swap.
	if os.Geteuid() == 0 {
		/*
			0x4 is MCL_ONFAULT, a new Linux kernel feature since 4.4. It prevents the significant virtual
			memory used by go runtime from occupying too much main memory.
			See https://github.com/golang/go/issues/28114 for more background information.
		*/
		if err := syscall.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE | 0x4); err == nil {
			logger.Warning("LockMemory", "", nil, "program has been locked into memory for safety reasons")
		} else {
			logger.Warning("LockMemory", "mlockall", err, "failed to lock memory")
		}
	} else {
		logger.Warning("LockMemory", "", nil, "program is not running as root (UID 0) hence memory cannot be locked, your private information may leak onto disk.")
	}
}
