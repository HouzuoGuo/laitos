//go:build darwin || linux
// +build darwin linux

package platform

import (
	"os"
	"syscall"
	"time"
)

// extProcAttr asks the new process to be placed in a group so that its child processes are also killed after timing out.
// There is no equivalent on Windows.
var extProcAttr = &syscall.SysProcAttr{Setpgid: true}

// GetRootDiskUsageKB returns used and total space of the file system mounted on /. Returns 0 if they cannot be determined.
func GetRootDiskUsageKB() (usedKB, freeKB, totalKB int64) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs("/", &fs)
	if err != nil {
		return
	}
	totalKB = int64(fs.Blocks) * int64(fs.Bsize) / 1024
	freeKB = int64(fs.Bfree) * int64(fs.Bsize) / 1024
	usedKB = totalKB - freeKB
	return
}

// KillProcess kills the process and its child processes. The function gives the processes a second to clean up after themselves.
func KillProcess(proc *os.Process) (success bool) {
	if proc == nil {
		return true
	}
	pid := proc.Pid
	if pid < 1 {
		return true
	}
	// Send SIGTERM to the process group (if any) and the process itself
	if killErr := syscall.Kill(-pid, syscall.SIGTERM); killErr == nil {
		success = true
	}
	if killErr := syscall.Kill(pid, syscall.SIGTERM); killErr == nil {
		success = true
	}
	// Wait a second for the process to clean up after itself, then force their termination.
	time.Sleep(1 * time.Second)
	if killErr := syscall.Kill(-pid, syscall.SIGKILL); killErr == nil {
		success = true
	}
	if killErr := syscall.Kill(pid, syscall.SIGKILL); killErr == nil {
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
			logger.Warning("", nil, "program has been locked into memory for safety reasons")
		} else {
			logger.Warning("mlockall", err, "failed to lock memory")
		}
	} else {
		logger.Warning("", nil, "program is not running as root (UID 0) hence memory cannot be locked, your private information may leak onto disk.")
	}
}

// Sync makes the sync syscall.
func Sync() {
	syscall.Sync()
}
