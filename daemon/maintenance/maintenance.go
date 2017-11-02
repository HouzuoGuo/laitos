package maintenance

import (
	"bytes"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/daemon/httpd/api"
	"github.com/HouzuoGuo/laitos/daemon/plainsocket"
	"github.com/HouzuoGuo/laitos/daemon/smtpd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/daemon/sockd"
	"github.com/HouzuoGuo/laitos/daemon/telegrambot"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
)

const (
	TCPPortCheckTimeoutSec = 10  // TCPPortCheckTimeoutSec is the timeout used in knocking ports.
	MinimumIntervalSec     = 120 // MinimumIntervalSec is the lowest acceptable value of system maintenance interval.
)

/*
Daemon is a system maintenance daemon that periodically triggers health check and software updates. Maintenance routine
comprises port checks, API key checks, and a lot more. Software updates ensures that system packages are up to date and
dependencies of this program are installed and up to date.
The result of each run is is sent to designated email addresses, along with latest environment information such as
latest logs and warnings.
*/
type Daemon struct {
	/*
		CheckTCPPorts are hosts and TCP port numbers to knock during the routine maintenance. If the port is not open on
		the host, the check is considered a failure.
	*/
	CheckTCPPorts map[string][]int `json:"CheckTCPPorts"`
	/*
		IntervalSec determines the rate of execution of maintenance routine. This is not a sleep duration. The constant
		rate of execution is maintained by taking away routine's elapsed time from actual interval between runs.
	*/
	IntervalSec        int                    `json:"IntervalSec"`
	MailClient         inet.MailClient        `json:"MailClient"` // Send notification mails via this mailer
	Recipients         []string               `json:"Recipients"` // Address of recipients of notification mails
	FeaturesToCheck    *toolbox.FeatureSet    `json:"-"`          // Health check subject - features and their API keys
	CheckMailCmdRunner *mailcmd.CommandRunner `json:"-"`          // Health check subject - mail processor and its mailer

	loopIsRunning int32     // Value is 1 only when maintenance loop is running
	stop          chan bool // Signal maintenance loop to stop
	logger        misc.Logger
}

/*
GetLatestStats returns statistic information from all front-end daemons, each on their own line.
Due to inevitable cyclic import, this function is defined twice, once in api.go of api package, the other in
maintenance.go of maintenance package.
*/
func GetLatestStats() string {
	numDecimals := 2
	factor := 1000000000.0
	return fmt.Sprintf(`Web and bot commands: %s
DNS server  TCP|UDP:  %s | %s
Web servers:          %s
Mail commands:        %s
Text server TCP|UDP:  %s | %s
Mail server:          %s
Sock server TCP|UDP:  %s | %s
Telegram commands:    %s
`,
		common.DurationStats.Format(factor, numDecimals),
		dnsd.TCPDurationStats.Format(factor, numDecimals), dnsd.UDPDurationStats.Format(factor, numDecimals),
		api.DurationStats.Format(factor, numDecimals),
		mailcmd.DurationStats.Format(factor, numDecimals),
		plainsocket.TCPDurationStats.Format(factor, numDecimals), plainsocket.UDPDurationStats.Format(factor, numDecimals),
		smtpd.DurationStats.Format(factor, numDecimals),
		sockd.TCPDurationStats.Format(factor, numDecimals), sockd.UDPDurationStats.Format(factor, numDecimals),
		telegrambot.DurationStats.Format(factor, numDecimals))
}

// runPortsCheck knocks on TCP ports that are to be checked in parallel, it returns an error if any of the ports fails to connect.
func (daemon *Daemon) runPortsCheck() error {
	portErrs := make([]string, 0, 0)
	portErrsMutex := new(sync.Mutex)
	wait := new(sync.WaitGroup)

	for host, ports := range daemon.CheckTCPPorts {
		if host == "" || ports == nil || len(ports) == 0 {
			continue
		}
		for _, port := range ports {
			wait.Add(1)
			go func(host string, port int) {
				// Expect connection to open very shortly
				dest := fmt.Sprintf("%s:%d", host, port)
				conn, err := net.DialTimeout("tcp", dest, TCPPortCheckTimeoutSec*time.Second)
				if err != nil {
					portErrsMutex.Lock()
					portErrs = append(portErrs, dest)
					portErrsMutex.Unlock()
				} else {
					conn.Close()
				}
				wait.Done()
			}(host, port)
		}
	}
	wait.Wait()
	if len(portErrs) == 0 {
		return nil
	}
	return fmt.Errorf("failed to connect to %s", strings.Join(portErrs, ", "))
}

// Check TCP ports and features, return all-OK or not.
func (daemon *Daemon) Execute() (string, bool) {
	daemon.logger.Printf("Execute", "", nil, "running now")
	// Conduct system maintenance first to ensure an accurate reading of runtime information later on
	maintResult := daemon.SystemMaintenance()
	// Do three checks in parallel - ports, features, and mail command runner
	var featureErrs map[toolbox.Trigger]error
	var mailCmdRunnerErr error
	var portsErr error
	waitAllChecks := new(sync.WaitGroup)
	waitAllChecks.Add(3) // will wait for port checks, feature tests, and mail command runner tests.
	go func() {
		// Port checks - the routine itself also uses concurrency internally
		portsErr = daemon.runPortsCheck()
		waitAllChecks.Done()
	}()
	go func() {
		// Feature self test - the routine itself also uses concurrency internally
		featureErrs = daemon.FeaturesToCheck.SelfTest()
		waitAllChecks.Done()
	}()
	go func() {
		// Mail command runner test - the routine itself also uses concurrency internally
		if daemon.CheckMailCmdRunner != nil {
			mailCmdRunnerErr = daemon.CheckMailCmdRunner.SelfTest()
		}
		waitAllChecks.Done()
	}()
	waitAllChecks.Wait()

	// Results are ready, time to compose mail body.
	allOK := len(featureErrs) == 0 && portsErr == nil && mailCmdRunnerErr == nil
	var result bytes.Buffer
	if allOK {
		result.WriteString("All OK\n")
	} else {
		result.WriteString("There are errors!!!\n")
	}
	// Latest runtime info
	result.WriteString(toolbox.GetRuntimeInfo())
	// Latest stats
	result.WriteString("\nDaemon stats - low/avg/high/total seconds and (count):\n")
	result.WriteString(GetLatestStats())
	// Port check results
	if portsErr == nil {
		result.WriteString("\nPorts: OK\n")
	} else {
		result.WriteString(fmt.Sprintf("\nPort errors: %v\n", portsErr))
	}
	// Feature check results
	if len(featureErrs) == 0 {
		result.WriteString("\nFeatures: OK\n")
	} else {
		for trigger, err := range featureErrs {
			result.WriteString(fmt.Sprintf("\nFeatures %s: %+v\n", trigger, err))
		}
	}
	// Mail command runner check results
	if mailCmdRunnerErr == nil {
		result.WriteString("\nMail processor: OK\n")
	} else {
		result.WriteString(fmt.Sprintf("\nMail processor: %v\n", mailCmdRunnerErr))
	}
	// Maintenance results, warnings, logs, and stack traces, in that order.
	result.WriteString("\nSystem maintenance:\n")
	result.WriteString(maintResult)
	result.WriteString("\nWarnings:\n")
	result.WriteString(toolbox.GetLatestWarnings())
	result.WriteString("\nLogs:\n")
	result.WriteString(toolbox.GetLatestLog())
	result.WriteString("\nStack traces:\n")
	result.WriteString(toolbox.GetGoroutineStacktraces())
	// Send away!
	if allOK {
		daemon.logger.Printf("Execute", "", nil, "completed with everything being OK")
	} else {
		daemon.logger.Warningf("Execute", "", nil, "completed with some errors")
	}
	if err := daemon.MailClient.Send(inet.OutgoingMailSubjectKeyword+"-maintenance", result.String(), daemon.Recipients...); err != nil {
		daemon.logger.Warningf("Execute", "", err, "failed to send notification mail")
	}
	// Remove weird characters that may appear and cause email display to squeeze all lines together
	var cleanedResult bytes.Buffer
	for _, r := range result.String() {
		if r < 128 && (unicode.IsPrint(r) || unicode.IsSpace(r)) {
			cleanedResult.WriteRune(r)
		} else {
			cleanedResult.WriteRune('?')
		}
	}
	return cleanedResult.String(), allOK
}

func (daemon *Daemon) Initialise() error {
	daemon.logger = misc.Logger{ComponentName: "maintenance", ComponentID: strconv.Itoa(daemon.IntervalSec)}
	if daemon.IntervalSec < MinimumIntervalSec {
		return fmt.Errorf("maintenance.StartAndBlock: IntervalSec must be at or above %d", MinimumIntervalSec)
	}
	daemon.stop = make(chan bool)
	return nil
}

/*
You may call this function only after having called Initialise()!
Start health check loop and block caller until Stop function is called.
*/
func (daemon *Daemon) StartAndBlock() error {
	firstTime := true
	// The very first health check is executed soon (10 minutes) after health check daemon starts up
	nextRunAt := time.Now().Add(10 * time.Minute)
	for {
		if misc.EmergencyLockDown {
			atomic.StoreInt32(&daemon.loopIsRunning, 0)
			return misc.ErrEmergencyLockDown
		}
		atomic.StoreInt32(&daemon.loopIsRunning, 1)
		if firstTime {
			select {
			case <-daemon.stop:
				atomic.StoreInt32(&daemon.loopIsRunning, 0)
				return nil
			case <-time.After(time.Until(nextRunAt)):
				nextRunAt = nextRunAt.Add(time.Duration(daemon.IntervalSec) * time.Second)
				daemon.Execute()
			}
			firstTime = false
		} else {
			// Afterwards, try to maintain a steady rate of execution.
			select {
			case <-daemon.stop:
				atomic.StoreInt32(&daemon.loopIsRunning, 0)
				return nil
			case <-time.After(time.Until(nextRunAt)):
				nextRunAt = nextRunAt.Add(time.Duration(daemon.IntervalSec) * time.Second)
				daemon.Execute()
			}
		}
	}
}

// Stop previously started health check loop.
func (daemon *Daemon) Stop() {
	if atomic.CompareAndSwapInt32(&daemon.loopIsRunning, 1, 0) {
		daemon.stop <- true
	}
}

/*
SystemMaintenance uses Linux package manager to ensure that all system packages are up to date and laitos dependencies
are installed and up to date. Returns human-readable result output.
*/
func (daemon *Daemon) SystemMaintenance() string {
	ret := new(bytes.Buffer)
	ret.WriteString("--- Conducting system maintenance...\n")
	// Find a system package manager
	var pkgManagerPath, pkgManagerName string
	for _, binPrefix := range []string{"/sbin", "/bin", "/usr/sbin", "/usr/bin", "/usr/sbin/local", "/usr/bin/local"} {
		// Prefer zypper over apt-get bacause opensuse has a weird "apt-get wrapper" that is not remotely functional
		for _, execName := range []string{"yum", "zypper", "apt-get"} {
			pkgManagerPath = path.Join(binPrefix, execName)
			if _, err := os.Stat(pkgManagerPath); err == nil {
				pkgManagerName = execName
				break
			}
		}
		if pkgManagerName != "" {
			break
		}
	}
	if pkgManagerName == "" {
		ret.WriteString("--- Cannot find a compatible package manager, abort.\n")
		return ret.String()
	} else {
		fmt.Fprintf(ret, "--- Package manage is %v\n", pkgManagerPath)
	}
	// Let package manager program inherit my environment variables, $PATH is especially important for apt.
	myEnv := os.Environ()
	pkgManagerEnv := make([]string, len(myEnv))
	copy(pkgManagerEnv, myEnv)
	// apt-get is too old to be convenient, it has to update the manifest first.
	if pkgManagerName == "apt-get" {
		ret.WriteString("--- Updating apt manifests...\n")
		pkgManagerEnv = append(pkgManagerEnv, "DEBIAN_FRONTEND=noninteractive")
		// Five minutes should be enough to grab the latest manifest
		daemon.logger.Printf("SystemMaintenance", "", nil, "updating apt manifests")
		result, err := misc.InvokeProgram(pkgManagerEnv, 5*60, pkgManagerPath, "update")
		daemon.logger.Printf("SystemMaintenance", "", err, "finished updating apt manifests")
		// There is no need to suppress this output according to markers
		fmt.Fprintf(ret, "--- apt-get result: %v - %s\n\n", err, strings.TrimSpace(result))
	}
	// Determine package manager invocation parameters
	var sysUpgradeArgs, installArgs []string
	switch pkgManagerName {
	case "yum":
		// yum is simple and easy
		sysUpgradeArgs = []string{"-y", "-t", "--skip-broken", "update"}
		installArgs = []string{"-y", "-t", "--skip-broken", "install"}
	case "apt-get":
		// apt is too old to be convenient
		sysUpgradeArgs = []string{"-q", "-y", "-f", "-m", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold", "upgrade"}
		installArgs = []string{"-q", "-y", "-f", "-m", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold", "install"}
	case "zypper":
		// zypper cannot English
		sysUpgradeArgs = []string{"--non-interactive", "update", "--recommends", "--auto-agree-with-licenses", "--skip-interactive", "--replacefiles", "--force-resolution"}
		installArgs = []string{"--non-interactive", "install", "--recommends", "--auto-agree-with-licenses", "--replacefiles", "--force-resolution"}
	default:
		fmt.Fprintf(ret, "--- Programming error: missing case for package manager %s\n", pkgManagerName)
		return ret.String()
	}
	// If package manager output contains any of the strings, the output is reduced into "Nothing to do"
	suppressOutputMarkers := []string{"No packages marked for update", "Nothing to do", "0 upgraded, 0 newly installed", "Unable to locate"}
	// Upgrade system packages with a time constraint of two hours
	ret.WriteString("--- Upgrading system packages...\n")
	daemon.logger.Printf("SystemMaintenance", "", nil, "updating system packages")
	result, err := misc.InvokeProgram(pkgManagerEnv, 2*3600, pkgManagerPath, sysUpgradeArgs...)
	daemon.logger.Printf("SystemMaintenance", "", err, "finished updating system packages")
	for _, marker := range suppressOutputMarkers {
		// If nothing was done during system update, suppress the rather useless output.
		if strings.Contains(result, marker) {
			result = "Nothing to do"
			break
		}
	}
	fmt.Fprintf(ret, "--- System upgrade result: %v - %s\n\n", err, strings.TrimSpace(result))
	/*
		Install additional software packages.
		laitos itself does not rely on any third-party library or program to run, however, it is very useful to install
		several utility applications to help out with system maintenance.
		Several of the package names are repeated under different names to accommodate the differences in naming convention
		among distributions.
	*/
	pkgs := []string{
		// Soft and hard dependencies of phantomJS
		"bzip2-libs", "cjkuni-fonts-common", "cjkuni-ukai-fonts", "cjkuni-uming-fonts", "dejavu-fonts-common",
		"dejavu-sans-fonts", "dejavu-serif-fonts", "expat", "fontconfig", "fontconfig-config", "fontpackages-filesystem",
		"fonts-arphic-ukai", "fonts-arphic-uming", "fonts-dejavu-core", "fonts-liberation", "freetype",
		"intlfonts-chinese-big-bitmap-fonts", "intlfonts-chinese-bitmap-fonts", "lib64z1", "libbz2-1", "libbz2-1.0",
		"liberation2-fonts", "liberation-fonts-common", "liberation-mono-fonts", "liberation-sans-fonts", "liberation-serif-fonts",
		"libexpat1", "libfontconfig1", "libfontenc", "libfreetype6", "libpng", "libpng16-16", "libXfont", "xorg-x11-fonts-Type1",
		"xorg-x11-font-utils", "zlib", "zlib1g",

		// Utility applications for time maintenance
		"chrony", "ntp", "ntpd", "ntpdate",
		// Utility applications for maintaining application zip bundle
		"unzip", "zip",
		// Utility applications for conducting network diagnosis
		"nc", "net-tools", "netcat", "nmap", "traceroute",
		// Utility box busybox is also useful for clock synchronisation
		"busybox",
	}
	/*
		Although all three package managers can install more than one packages at a time, the packages are still
		installed one after another, because:
		- apt-get does not ignore non-existent package names, how inconvenient.
		- if zypper runs into unsatisfactory package dependencies, it aborts the whole installation.
		yum is once again the superior solution among all three.
	*/
	for _, name := range pkgs {
		// Put software name next to installation parameters
		pkgInstallArgs := make([]string, len(installArgs)+1)
		copy(pkgInstallArgs, installArgs)
		pkgInstallArgs[len(installArgs)] = name
		fmt.Fprintf(ret, "--- Installing/upgrading %s\n", name)
		// Five minutes should be good enough for every package
		daemon.logger.Printf("SystemMaintenance", "", nil, "installing package %s", name)
		result, err := misc.InvokeProgram(pkgManagerEnv, 5*60, pkgManagerPath, pkgInstallArgs...)
		daemon.logger.Printf("SystemMaintenance", "", err, "finished installing package %s", name)
		for _, marker := range suppressOutputMarkers {
			// If nothing was done about the package, suppress the rather useless output.
			if strings.Contains(result, marker) {
				result = "Nothing to do"
				break
			}
		}
		fmt.Fprintf(ret, "--- %s installation/upgrade result: %v - %s\n\n", name, err, strings.TrimSpace(result))
	}
	// Use three tools to immediately synchronise system clock
	result, err = misc.InvokeProgram(nil, 60, "ntpdate", "-4", "0.pool.ntp.org", "us.pool.ntp.org", "de.pool.ntp.org", "nz.pool.ntp.org")
	fmt.Fprintf(ret, "--- clock synchronisation result (ntpdate): %v - %s\n\n", err, strings.TrimSpace(result))
	result, err = misc.InvokeProgram(nil, 60, "chronyd", "-q", "pool pool.ntp.org iburst")
	fmt.Fprintf(ret, "--- clock synchronisation result (chronyd): %v - %s\n\n", err, strings.TrimSpace(result))
	result, err = misc.InvokeProgram(nil, 60, "busybox", "ntpd", "-n", "-q", "-p", "ie.pool.ntp.org", "ca.pool.ntp.org", "au.pool.ntp.org")
	fmt.Fprintf(ret, "--- clock synchronisation result (busybox): %v - %s\n\n", err, strings.TrimSpace(result))
	/*
		The program startup time is used to detect outdated commands (such as in telegram bot), in rare case if system clock
		was severely skewed, causing program startup time to be in the future, the detection mechanisms will misbehave.
		Therefore, correct the situation here.
	*/
	if misc.StartupTime.After(time.Now()) {
		fmt.Fprint(ret, "--- clock was severely skewed, reset program startup time.\n")
		// The earliest possible opportunity for system maintenance to run is now minus minimum interval
		misc.StartupTime = time.Now().Add(-MinimumIntervalSec * time.Second)
	}

	ret.WriteString("--- System maintenance has finished.\n")
	daemon.logger.Printf("SystemMaintenance", "", nil, "maintenance is finished")
	return ret.String()
}

// Run unit tests on the maintenance daemon. See TestMaintenance_Execute for daemon setup.
func TestMaintenance(check *Daemon, t testingstub.T) {
	// Hopefully nothing is listening on that port
	check.CheckTCPPorts = map[string][]int{"localhost": {11334}}
	// If it fails, the failure could only come from mailer of mail processor.
	if result, ok := check.Execute(); ok || !strings.Contains(result, "Port errors") {
		t.Fatal(result)
	}
	// Listen on the port and test port knocking along with other checks
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		if _, err := listener.Accept(); err != nil {
			return
		}
	}()
	time.Sleep(1 * time.Second)
	check.CheckTCPPorts = map[string][]int{"localhost": {listener.Addr().(*net.TCPAddr).Port}}
	// If it fails, the failure could only come from mailer of mail processor.
	if result, ok := check.Execute(); !ok && !strings.Contains(result, "MailClient.SelfTest") {
		t.Fatal(result)
	}
	// Break a feature
	check.FeaturesToCheck.LookupByTrigger[".s"] = &toolbox.Shell{}
	if result, ok := check.Execute(); ok || !strings.Contains(result, ".s") {
		t.Fatal(result)
	}
	check.FeaturesToCheck.LookupByTrigger[".s"] = &toolbox.Shell{InterpreterPath: "/bin/bash"}
	// Expect checks to begin within a second
	if err := check.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Maintenance loop should successfully start within two seconds
	var stoppedNormally bool
	go func() {
		if err := check.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stoppedNormally = true
	}()
	time.Sleep(2 * time.Second)
	// Daemon must stop in a second
	check.Stop()
	time.Sleep(1 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	check.Stop()
	check.Stop()
}
