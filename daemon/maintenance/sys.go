package maintenance

import (
	"bytes"
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"strings"
	"time"
)

// SynchroniseSystemClock uses three different tools to immediately synchronise system clock via NTP servers.
func (daemon *Daemon) SynchroniseSystemClock(out *bytes.Buffer) {
	daemon.logPrintStage(out, "synchronise clock")
	// Use three tools to immediately synchronise system clock
	result, err := misc.InvokeProgram([]string{"PATH=" + misc.CommonPATH}, 60, "ntpdate", "-4", "0.pool.ntp.org", "us.pool.ntp.org", "de.pool.ntp.org", "nz.pool.ntp.org")
	daemon.logPrintStageStep(out, "ntpdate: %v - %s", err, strings.TrimSpace(result))
	result, err = misc.InvokeProgram([]string{"PATH=" + misc.CommonPATH}, 60, "chronyd", "-q", "pool pool.ntp.org iburst")
	daemon.logPrintStageStep(out, "chronyd: %v - %s", err, strings.TrimSpace(result))
	result, err = misc.InvokeProgram([]string{"PATH=" + misc.CommonPATH}, 60, "busybox", "ntpd", "-n", "-q", "-p", "ie.pool.ntp.org", "ca.pool.ntp.org", "au.pool.ntp.org")
	daemon.logPrintStageStep(out, "busybox ntpd: %v - %s", err, strings.TrimSpace(result))
	/*
		The program startup time is used to detect outdated commands (such as in telegram bot), in rare case if system clock
		was severely skewed, causing program startup time to be in the future, the detection mechanisms will misbehave.
		Therefore, correct the situation here.
	*/
	if misc.StartupTime.After(time.Now()) {
		daemon.logPrintStageStep(out, "clock was severely skewed, reset program startup time.")
		// The earliest possible opportunity for system maintenance to run is now minus initial delay
		misc.StartupTime = time.Now().Add(-InitialDelaySec * time.Second)
	}
	fmt.Fprint(out, "\n")
}

// MaintainServices manipulate service state according to configuration.
func (daemon *Daemon) MaintainServices(out *bytes.Buffer) {
	if daemon.DisableStopServices == nil && daemon.EnableStartServices == nil {
		return
	}
	daemon.logPrintStage(out, "maintain service state")

	if daemon.DisableStopServices != nil {
		for _, name := range daemon.DisableStopServices {
			if !misc.DisableStopDaemon(name) {
				daemon.logPrintStageStep(out, "disable&stop %s: success? false", name)
			}
		}
	}
	if daemon.EnableStartServices != nil {
		for _, name := range daemon.EnableStartServices {
			if !misc.EnableStartDaemon(name) {
				daemon.logPrintStageStep(out, "enable&start %s: success? false", name)
			}
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
