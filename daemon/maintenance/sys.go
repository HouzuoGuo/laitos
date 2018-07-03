package maintenance

import (
	"bytes"
	"github.com/HouzuoGuo/laitos/misc"
	"strings"
	"time"
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
		for _, name := range daemon.DisableStopServices {
			daemon.logPrintStageStep(out, "disable&stop %s: success? %v", name, misc.DisableStopDaemon(name))
		}
	}
	if daemon.EnableStartServices != nil {
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
