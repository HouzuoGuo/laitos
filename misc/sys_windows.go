package misc

import (
	"os"
	"os/exec"
	"strconv"
)

// GetRootDiskUsageKB returns used and total space of the file system mounted on /. Returns 0 if they cannot be determined.
func GetRootDiskUsageKB() (usedKB, freeKB, totalKB int) {
	return 0, 0, 0
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
