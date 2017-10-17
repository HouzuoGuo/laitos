package launcher

import (
	"strings"
)

const (
	// Daemon names
	DNSDName          = "dnsd"
	HTTPDName         = "httpd"
	InsecureHTTPDName = "insecurehttpd"
	MaintenanceName   = "maintenance"
	PlainSocketName   = "plainsocket"
	SMTPDName         = "smtpd"
	SOCKDName         = "sockd"
	TelegramName      = "telegram"
)

// AllDaemons is an unsorted list of string daemon names.
var AllDaemons = []string{DNSDName, HTTPDName, InsecureHTTPDName, MaintenanceName, PlainSocketName, SMTPDName, SOCKDName, TelegramName}

// ShedOrder is the sequence of daemon names to be taken offline one after another in case of program crash.
var ShedOrder = []string{MaintenanceName, DNSDName, SOCKDName, SMTPDName, HTTPDName, InsecureHTTPDName, PlainSocketName, TelegramName}

// removeFromFlags removes CLI flag from input flags base on a condition function (true to remove).
func removeFromFlags(condition func(string) bool, flags []string) (ret []string) {
	ret = make([]string, 0, len(flags))
	var absorbNext bool
	for _, str := range flags {
		if condition(str) {
			if !strings.Contains(str, "=") {
				absorbNext = true
			}
		} else if absorbNext {
			absorbNext = false
		} else {
			ret = append(ret, str)
		}
	}
	return
}

/*
Supervisor manages the lifecycle of laitos main program that runs daemons. In case of main program crash, the supervisor
relaunches the main program using reduced number of daemons, thus ensuring maximum availability of healthy daemons.
*/
type Supervisor struct {
	// CLIArgs are the thorough list of original program arguments to launch laitos. This includes the leading executable path.
	CLIArgs []string
	// Config is laitos configuration deserialised from user's config JSON file.
	Config Config
	// DaemonNames are the original set of daemon names that user asked to start.
	DaemonNames []string
	// ShedSequence is the sequence at which daemon shedding takes place. Each latter array has one daemon less than the previous.
	ShedSequence [][]string
}

// makeShedSequence constructs the remaining daemon names of each shedding round.
func (sup *Supervisor) makeShedSequence() {
	sup.ShedSequence = make([][]string, 0, len(sup.DaemonNames))
	remainingDaemons := sup.DaemonNames
	for _, toShed := range ShedOrder {
		// Do not shed the very last daemon
		if len(remainingDaemons) == 1 {
			break
		}
		// Each round has one less daemon in comparison to the previous round
		thisRound := make([]string, 0)
		var willShed bool
		for _, daemon := range remainingDaemons {
			if daemon == toShed {
				willShed = true
			} else {
				thisRound = append(thisRound, daemon)
			}
		}
		if willShed {
			remainingDaemons = thisRound
			sup.ShedSequence = append(sup.ShedSequence, thisRound)
		}
	}
}

func (sup *Supervisor) Start() {
	for i := 0; ; i++ {
	}
}

// GetLaunchParameters returns the parameters used for launching laitos program for the N-th attempt.
func (sup *Supervisor) GetLaunchParameters(nthAttempt int) (cliArgs []string, daemonNames []string) {
	cliArgs = make([]string, 0, len(sup.CLIArgs))
	copy(cliArgs, sup.CLIArgs)
	if nthAttempt <= 0 {
		// The first attempt is a normal start, it tells laitos not to run supervisor.
		cliArgs = removeFromFlags(func(f string) bool {
			return strings.HasPrefix(f, "-supervisor")
		}, cliArgs)
		cliArgs = append(cliArgs, "-supervisor=false")
	}
	if nthAttempt <= 1 {
		/*
			The second attempt removes all but essential program flags (-config and -daemons), this means system
			environment will not be altered by the advanced start option such as -gomaxprocs and -tunesystem.
		*/
		cliArgs = removeFromFlags(func(f string) bool {
			return !strings.HasPrefix(f, "-config") && !strings.HasPrefix(f, "-daemons")
		}, cliArgs)
	}
	if nthAttempt > 1 && nthAttempt < len(sup.DaemonNames)+1 {
		// Further attempts will shed daemons
		daemonNames = sup.ShedSequence[nthAttempt-1]
	}
	if nthAttempt > len(sup.DaemonNames)+1 {
		// More attempts will just return original lists
		copy(cliArgs, sup.CLIArgs)
		copy(daemonNames, sup.DaemonNames)
	}
	return
}
