package maintenance

import (
	"bytes"
	"github.com/HouzuoGuo/laitos/misc"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	SwapFilePath = "/laitos-swap-file"
)

// SynchroniseSystemClock uses three different tools to immediately synchronise system clock via NTP servers.
func (daemon *Daemon) SynchroniseSystemClock(out *bytes.Buffer) {
	if misc.HostIsWindows() {
		daemon.logPrintStage(out, "skipped on windows: synchronise clock")
		daemon.CorrectStartupTime(out)
		return
	}
	daemon.logPrintStage(out, "synchronise clock")
	// Use three tools to immediately synchronise system clock
	result, err := misc.InvokeProgram([]string{"PATH=" + misc.CommonPATH}, 60, "ntpdate", "-4", "0.pool.ntp.org", "us.pool.ntp.org", "de.pool.ntp.org", "nz.pool.ntp.org")
	daemon.logPrintStageStep(out, "ntpdate: %v - %s", err, strings.TrimSpace(result))
	result, err = misc.InvokeProgram([]string{"PATH=" + misc.CommonPATH}, 60, "chronyd", "-q", "pool pool.ntp.org iburst")
	daemon.logPrintStageStep(out, "chronyd: %v - %s", err, strings.TrimSpace(result))
	result, err = misc.InvokeProgram([]string{"PATH=" + misc.CommonPATH}, 60, "busybox", "ntpd", "-n", "-q", "-p", "ie.pool.ntp.org", "ca.pool.ntp.org", "au.pool.ntp.org")
	daemon.logPrintStageStep(out, "busybox ntpd: %v - %s", err, strings.TrimSpace(result))

	daemon.CorrectStartupTime(out)
}

/*
CorrectStartTime corrects program start time in case system clock is skewed.
The program startup time is used to detect outdated commands (such as in telegram bot), in rare case if system clock
was severely skewed, causing program startup time to be in the future, the detection mechanisms will misbehave.
*/
func (daemon *Daemon) CorrectStartupTime(out *bytes.Buffer) {
	if misc.StartupTime.After(time.Now()) {
		daemon.logPrintStage(out, "clock was severely skewed, reset program startup time.")
		// The earliest possible opportunity for system maintenance to run is now minus initial delay
		misc.StartupTime = time.Now().Add(-InitialDelaySec * time.Second)
	}
}

// MaintainServices manipulate service state according to configuration.
func (daemon *Daemon) MaintainServices(out *bytes.Buffer) {
	if daemon.DisableStopServices == nil && daemon.EnableStartServices == nil {
		return
	}
	daemon.logPrintStage(out, "maintain service state")

	if daemon.DisableStopServices != nil {
		sort.Strings(daemon.DisableStopServices)
		for _, name := range daemon.DisableStopServices {
			daemon.logPrintStageStep(out, "disable&stop %s: success? %v", name, misc.DisableStopDaemon(name))
		}
	}
	if daemon.EnableStartServices != nil {
		sort.Strings(daemon.EnableStartServices)
		for _, name := range daemon.EnableStartServices {
			daemon.logPrintStageStep(out, "enable&start %s: success? %v", name, misc.EnableStartDaemon(name))
		}
	}
}

// BlockUnusedLogin will block/disable system login from users not listed in the exception list.
func (daemon *Daemon) BlockUnusedLogin(out *bytes.Buffer) {
	if daemon.BlockSystemLoginExcept == nil || len(daemon.BlockSystemLoginExcept) == 0 {
		return
	}
	daemon.logPrintStage(out, "block unused system login")
	// Exception name list is case insensitive
	exceptionMap := make(map[string]bool)
	for _, name := range daemon.BlockSystemLoginExcept {
		exceptionMap[strings.ToLower(name)] = true
	}
	for userName := range misc.GetLocalUserNames() {
		if exceptionMap[strings.ToLower(userName)] {
			daemon.logPrintStageStep(out, "not going to touch excluded user %s", userName)
			continue
		}
		if ok, blockOut := misc.BlockUserLogin(userName); ok {
			daemon.logPrintStageStep(out, "blocked user %s", userName)
		} else {
			daemon.logPrintStageStep(out, "failed to block user %s - %v", userName, blockOut)
		}
	}
}

// MaintainWindowsIntegrity uses DISM and FSC utilities to maintain Windows system integrity.
func (daemon *Daemon) MaintainWindowsIntegrity(out *bytes.Buffer) {
	if !misc.HostIsWindows() {
		return
	}
	daemon.logPrintStage(out, "maintain windows system integrity")
	// These tools seriously spend a lot of time
	progOut, err := misc.InvokeProgram(nil, 3*3600, `C:\Windows\system32\Dism.exe`, "/Online", "/Cleanup-Image", "/StartComponentCleanup", "/ResetBase")
	daemon.logPrintStageStep(out, "dism StartComponentCleanup: %v - %s", err, progOut)
	progOut, err = misc.InvokeProgram(nil, 3*3600, `C:\Windows\system32\Dism.exe`, "/Online", "/Cleanup-Image", "/SPSuperseded")
	daemon.logPrintStageStep(out, "dism SPSuperseded: %v - %s", err, progOut)
	progOut, err = misc.InvokeProgram(nil, 3*3600, `C:\Windows\system32\Dism.exe`, "/Online", "/Cleanup-Image", "/Restorehealth")
	daemon.logPrintStageStep(out, "dism Restorehealth: %v - %s", err, progOut)
	progOut, err = misc.InvokeProgram(nil, 3*3600, `C:\Windows\system32\sfc.exe`, "/ScanNow")
	daemon.logPrintStageStep(out, "sfc ScanNow: %v - %s", err, progOut)
}

// MaintainSwapFile creates and activates a swap file for Linux system, or turns swap off depending on configuration input.
func (daemon *Daemon) MaintainSwapFile(out *bytes.Buffer) {
	if misc.HostIsWindows() {
		daemon.logPrintStage(out, "skipped on windows: maintain swap file")
		return
	}
	if daemon.SwapFileSizeMB == 0 {
		return
	}
	daemon.logPrintStage(out, "create/turn on swap file "+SwapFilePath)
	if daemon.SwapFileSizeMB < 0 {
		daemon.logPrintStageStep(out, "turn off swap")
		if err := misc.SwapOff(); err != nil {
			daemon.logPrintStageStep(out, "failed to turn off swap: %v", err)
		}
		return
	} else if daemon.SwapFileSizeMB > 0 {
		_, swapFileStatus := os.Stat(SwapFilePath)
		// Create the swap file if it does not yet exist
		if os.IsNotExist(swapFileStatus) {
			buf := make([]byte, 1048576)
			fh, err := os.Create(SwapFilePath)
			if err != nil {
				daemon.logPrintStageStep(out, "failed to create swap file - %v", err)
				return
			}
			for i := 0; i < daemon.SwapFileSizeMB; i++ {
				if _, err := fh.Write(buf); err != nil {
					daemon.logPrintStageStep(out, "failed to create swap file - %v", err)
					return
				}
			}
			if err := fh.Sync(); err != nil {
				daemon.logPrintStageStep(out, "failed to create swap file - %v", err)
				return
			}
			if err := fh.Close(); err != nil {
				daemon.logPrintStageStep(out, "failed to create swap file - %v", err)
				return
			}
			// If the file already exists, it will not be grown or recreated.
		} else if swapFileStatus != nil {
			daemon.logPrintStageStep(out, "failed to determine swap file status - %v", swapFileStatus)
			return
		} else {
			daemon.logPrintStage(out, "the swap file appears to already exist")
		}
		// Correct the swap file permission and ownership
		if err := os.Chmod(SwapFilePath, 0600); err != nil {
			daemon.logPrintStageStep(out, "failed to correct swap file permission - %v", err)
			return
		}
		if err := os.Chown(SwapFilePath, 0, 0); err != nil {
			daemon.logPrintStageStep(out, "failed to correct swap file owner - %v", err)
			return
		}
		// Format the swap file
		if progOut, err := misc.InvokeProgram(nil, misc.CommonOSCmdTimeoutSec, "mkswap", SwapFilePath); err != nil {
			daemon.logPrintStageStep(out, "failed to format swap file - %v - %s", err, progOut)
		}
		// Turn on the swap file
		progOut, err := misc.InvokeProgram(nil, misc.CommonOSCmdTimeoutSec, "swapon", SwapFilePath)
		if err != nil {
			daemon.logPrintStageStep(out, "failed to turn on swap file - %v - %s", err, progOut)
		}
	}
}

// MaintainFileSystem gets rid of unused temporary files.
func (daemon *Daemon) MaintainFileSystem(out *bytes.Buffer) {
	daemon.logPrintStage(out, "maintain file system")
	// Remove files from temporary locations that have not been modified for over a week
	daemon.logPrintStageStep(out, "clean up unused temporary files")
	sevenDaysAgo := time.Now().Add(-(7 * 24 * time.Hour))
	// Keep in mind that /var/tmp is supposed to hold "persistent temporary files" in Linux
	for _, location := range []string{`/tmp`, `C:\Temp`, `C:\Windows\Temp`} {
		filepath.Walk(location, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				if info.ModTime().Before(sevenDaysAgo) {
					toDelete := filepath.Join(path, info.Name())
					if deleteErr := os.RemoveAll(toDelete); deleteErr == nil {
						daemon.logPrintStageStep(out, "deleted %s", toDelete)
					} else {
						daemon.logPrintStageStep(out, "failed to deleted %s - %v", toDelete, deleteErr)
					}
				}
			}
			return nil
		})
	}
}
