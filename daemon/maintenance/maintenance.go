package maintenance

import (
	"bytes"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/autounlock"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/daemon/httpd"
	"github.com/HouzuoGuo/laitos/daemon/httpd/handler"
	"github.com/HouzuoGuo/laitos/daemon/plainsocket"
	"github.com/HouzuoGuo/laitos/daemon/smtpd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/daemon/sockd"
	"github.com/HouzuoGuo/laitos/daemon/telegrambot"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	TCPPortCheckTimeoutSec = 10   // TCPPortCheckTimeoutSec is the timeout used in knocking ports.
	MinimumIntervalSec     = 3600 // MinimumIntervalSec is the lowest acceptable value of system maintenance interval.
	InitialDelaySec        = 60   // InitialDelaySec is the number of seconds to wait for the first maintenance run.
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
		BlockSystemLoginExcept is a list of Unix user names. If the array is not empty, system maintenance routine will
		disable login access to all local users except the names among the array in an effort to harden system security.
	*/
	BlockSystemLoginExcept []string `json:"BlockSystemLoginExcept"`
	// DisableStopServices is an array of system services to be stopped, disabled, and prevented from starting again.
	DisableStopServices []string `json:"DisableStopServices"`
	// EnableStartServices is an array of system services to be enabled and restarted.
	EnableStartServices []string `json:"EnableStartServices"`
	// InstallPackages is an array of software packages to be installed and upgraded.
	InstallPackages []string `json:"InstallPackages"`
	// BlockPortsExcept is an array of TCP and UDP ports to be blocked via iptables. Must be used in conjunction with ThrottleIncomingPackets.
	BlockPortsExcept []int `json:"BlockPortsExcept"`
	// ThrottleIncomingConnections throttles incoming connections and other network packets to this number/second via iptables.
	ThrottleIncomingPackets int `json:"ThrottleIncomingPackets"`
	// TuneLinux enables Linux kernel tuning routine as a maintenance step
	TuneLinux bool `json:"TuneLinux"`
	// SwapOff turns off swap as a maintenance step
	SwapOff bool `json:"SwapOff"`
	// SetTimeZone changes system time zone to the specified value (such as "UTC").
	SetTimeZone string `json:"SetTimeZone"`

	/*
		IntervalSec determines the rate of execution of maintenance routine. This is not a sleep duration. The constant
		rate of execution is maintained by taking away routine's elapsed time from actual interval between runs.
	*/
	IntervalSec         int                     `json:"IntervalSec"`
	MailClient          inet.MailClient         `json:"MailClient"` // Send notification mails via this mailer
	Recipients          []string                `json:"Recipients"` // Address of recipients of notification mails
	FeaturesToTest      *toolbox.FeatureSet     `json:"-"`          // FeaturesToTest are toolbox features to be tested during health check.
	MailCmdRunnerToTest *mailcmd.CommandRunner  `json:"-"`          // MailCmdRunnerToTest is mail command runner to be tested during health check.
	HTTPHandlersToCheck httpd.HandlerCollection `json:"-"`          // HTTPHandlersToCheck are the URL handlers of an HTTP daemon to be tested during health check.

	loopIsRunning int32     // Value is 1 only when maintenance loop is running
	stop          chan bool // Signal maintenance loop to stop
	logger        misc.Logger
}

/*
GetLatestStats returns statistic information from all front-end daemons, each on their own line.
Due to inevitable cyclic import, this function is defined twice, once in handler.go of handler package, the other in
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
Auto-unlock events:   %s
Outstanding mails:    %d KB
`,
		common.DurationStats.Format(factor, numDecimals),
		dnsd.TCPDurationStats.Format(factor, numDecimals), dnsd.UDPDurationStats.Format(factor, numDecimals),
		handler.DurationStats.Format(factor, numDecimals),
		mailcmd.DurationStats.Format(factor, numDecimals),
		plainsocket.TCPDurationStats.Format(factor, numDecimals), plainsocket.UDPDurationStats.Format(factor, numDecimals),
		smtpd.DurationStats.Format(factor, numDecimals),
		sockd.TCPDurationStats.Format(factor, numDecimals), sockd.UDPDurationStats.Format(factor, numDecimals),
		telegrambot.DurationStats.Format(factor, numDecimals),
		autounlock.UnlockStats.Format(factor, numDecimals),
		inet.OutstandingMailBytes/1024)
}

// runPortsCheck knocks on TCP ports that are to be checked in parallel, it returns an error if any of the ports fails to connect.
func (daemon *Daemon) runPortsCheck() error {
	if daemon.CheckTCPPorts == nil {
		return nil
	}

	portErrs := make([]string, 0, 0)
	portErrsMutex := new(sync.Mutex)
	wait := new(sync.WaitGroup)

	for host, ports := range daemon.CheckTCPPorts {
		if host == "" || ports == nil || len(ports) == 0 {
			continue
		}
		for _, port := range ports {
			if port == 25 && (inet.IsGCE() || inet.IsAzure()) {
				daemon.logger.Info("runPortsCheck", "", nil, "because Google and Azure cloud forbid connection to port 25, port check will skip %s:25", host)
				continue
			}
			wait.Add(1)
			go func(host string, port int) {
				// Expect connection to open very shortly
				dest := net.JoinHostPort(host, strconv.Itoa(port))
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
	daemon.logger.Info("Execute", "", nil, "running now")
	// Conduct system maintenance first to ensure an accurate reading of runtime information later on
	maintResult := daemon.SystemMaintenance()
	// Do three checks in parallel - ports, toolbox features, and mail command runner
	var portsErr, featureErr, mailCmdRunnerErr, httpHandlersErr error
	waitAllChecks := new(sync.WaitGroup)
	waitAllChecks.Add(4) // will wait for port checks, feature tests, mail command runner, and HTTP handler tests.
	go func() {
		// Port checks - the routine itself also uses concurrency internally
		portsErr = daemon.runPortsCheck()
		waitAllChecks.Done()
	}()
	go func() {
		// Toolbox feature self test - the routine itself also uses concurrency internally
		if daemon.FeaturesToTest != nil {
			featureErr = daemon.FeaturesToTest.SelfTest()
		}
		waitAllChecks.Done()
	}()
	go func() {
		// Mail command runner test - the routine itself also uses concurrency internally
		if daemon.MailCmdRunnerToTest != nil && daemon.MailCmdRunnerToTest.ReplyMailClient.IsConfigured() {
			mailCmdRunnerErr = daemon.MailCmdRunnerToTest.SelfTest()
		}
		waitAllChecks.Done()
	}()
	go func() {
		// HTTP special handler test - the routine itself also uses concurrency internally
		if daemon.HTTPHandlersToCheck != nil {
			httpHandlersErr = daemon.HTTPHandlersToCheck.SelfTest()
		}
		waitAllChecks.Done()
	}()

	waitAllChecks.Wait()

	// Results are now ready. When composing the mail body, place the most important&interesting piece of information at top.
	allOK := portsErr == nil && featureErr == nil && mailCmdRunnerErr == nil && httpHandlersErr == nil
	var result bytes.Buffer
	if allOK {
		result.WriteString("All OK\n")
	} else {
		result.WriteString("There are errors!!!\n")
	}
	result.WriteString(toolbox.GetRuntimeInfo())
	result.WriteString("\nDaemon stats - low/avg/high/total seconds and (count):\n")
	result.WriteString(GetLatestStats())
	if portsErr == nil {
		result.WriteString("\nPorts: OK\n")
	} else {
		result.WriteString(fmt.Sprintf("\nPort errors: %v\n", portsErr))
	}
	if featureErr == nil {
		result.WriteString("\nFeatures: OK\n")
	} else {
		result.WriteString(fmt.Sprintf("\nFeature errors: %v\n", featureErr))
	}
	if mailCmdRunnerErr == nil {
		result.WriteString("\nMail processor (if present): OK\n")
	} else {
		result.WriteString(fmt.Sprintf("\nMail processor errors: %v\n", mailCmdRunnerErr))
	}
	if httpHandlersErr == nil {
		result.WriteString("\nHTTP handlers (if present): OK\n")
	} else {
		result.WriteString(fmt.Sprintf("\nHTTP handler errors: %v\n", httpHandlersErr))
	}
	result.WriteString("\nWarnings:\n")
	result.WriteString(toolbox.GetLatestWarnings())
	result.WriteString("\nLogs:\n")
	result.WriteString(toolbox.GetLatestLog())
	result.WriteString("\nSystem maintenance:\n")
	result.WriteString(maintResult)
	result.WriteString("\nStack traces:\n")
	result.WriteString(toolbox.GetGoroutineStacktraces())
	// Send away!
	if allOK {
		daemon.logger.Info("Execute", "", nil, "completed with everything being OK")
	} else {
		daemon.logger.Warning("Execute", "", nil, "completed with some errors")
	}
	if daemon.Recipients == nil || len(daemon.Recipients) == 0 {
		// If there are no recipients, print the report to standard output.
		daemon.logger.Info("Execute", "", nil, "report will now be printed to standard output")
		fmt.Println("Maintenance report:")
		fmt.Println(result.String())
	} else if err := daemon.MailClient.Send(inet.OutgoingMailSubjectKeyword+"-maintenance", result.String(), daemon.Recipients...); err != nil {
		daemon.logger.Warning("Execute", "", err, "failed to send notification mail")
	}
	return inet.LintMailBody(result.String()), allOK
}

func (daemon *Daemon) Initialise() error {
	if daemon.IntervalSec < 1 {
		daemon.IntervalSec = 24 * 3600 // quite reasonable to run maintenance daily
	} else if daemon.IntervalSec < MinimumIntervalSec {
		return fmt.Errorf("maintenance.StartAndBlock: IntervalSec must be at or above %d", MinimumIntervalSec)
	}
	daemon.stop = make(chan bool)
	daemon.logger = misc.Logger{ComponentName: "maintenance", ComponentID: []misc.LoggerIDField{{"Intv", daemon.IntervalSec}}}
	return nil
}

/*
You may call this function only after having called Initialise()!
Start health check loop and block caller until Stop function is called.
*/
func (daemon *Daemon) StartAndBlock() error {
	firstTime := true
	// Maintenance is run for the very first time soon (2 minutes) after starting up
	nextRunAt := time.Now().Add(InitialDelaySec * time.Second)
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

// logPrintStage reports the start/finish of a maintenance stage to output buffer and log.
func (daemon *Daemon) logPrintStage(out *bytes.Buffer, template string, a ...interface{}) {
	fmt.Fprintf(out, "\n---"+template+"\n", a...)
	daemon.logger.Info("maintenance", "", nil, "Stage: "+template, a...)
}

// logPrintStage reports the start/finish of a maintenance step to output buffer and log.
func (daemon *Daemon) logPrintStageStep(out *bytes.Buffer, template string, a ...interface{}) {
	fmt.Fprintf(out, "---"+template+"\n", a...)
	daemon.logger.Info("maintenance", "", nil, "Step: "+template, a...)
}

// SystemMaintenance is a long routine that conducts comprehensive general system maintenance tasks.
func (daemon *Daemon) SystemMaintenance() string {
	out := new(bytes.Buffer)
	daemon.logPrintStage(out, "begin system maintenance")

	if daemon.TuneLinux {
		daemon.logPrintStage(out, "tune linux kernel: %s", toolbox.TuneLinux())
	}

	if daemon.SwapOff {
		daemon.logPrintStage(out, "turn off swap")
		if err := misc.SwapOff(); err != nil {
			daemon.logPrintStageStep(out, "failed to turn off swap: %v", err)
		}
	}

	if daemon.SetTimeZone != "" {
		daemon.logPrintStage(out, "set system time zone")
		if err := misc.SetTimeZone(daemon.SetTimeZone); err != nil {
			daemon.logPrintStageStep(out, "failed to set time zone: %v", err)
		}
	}

	/*
		It is usually only necessary to copy the utilities once, but on AWS ElasticBeanstalk the OS template
		aggressively clears /tmp at regular interval, losing all of the copied utilities in the progress, therefore
		re-copy the utility programs during maintenance.
	*/
	daemon.logPrintStage(out, "re-copy non-essential laitos utilities")
	misc.PrepareUtilities(daemon.logger)

	// General security tasks
	daemon.BlockUnusedLogin(out)
	daemon.MaintainServices(out)
	daemon.MaintainsIptables(out) // run this after service maintenance, because disabling firewall service may alter iptables.

	// Software installation tasks
	daemon.PrepareDockerRepositoryForDebian(out)
	daemon.UpgradeInstallSoftware(out)

	// Clock synchronisation may depend on a software installed via the previous step
	daemon.SynchroniseSystemClock(out)

	daemon.logPrintStage(out, "concluded system maintenance")
	return out.String()
}

// MaintainServices manipulate service state according to configuration.
func (daemon *Daemon) MaintainServices(out *bytes.Buffer) {
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

/*
PrepareDockerRepositorForDebian prepares APT repository for installing debian, because debian does not distribute
docker in their repository for whatever reason. If the system is not a debian the function will do nothing.
*/
func (daemon *Daemon) PrepareDockerRepositoryForDebian(out *bytes.Buffer) {
	daemon.logPrintStage(out, "prepare docker repository for debian")
	content, err := ioutil.ReadFile("/etc/os-release")
	if err != nil {
		daemon.logPrintStageStep(out, "failed to read os-release, this is not a critical error.")
		return
	} else if !strings.Contains(strings.ToLower(string(content)), "debian") || strings.Contains(strings.ToLower(string(content)), "ubuntu") {
		daemon.logPrintStageStep(out, "system is not a debian, just FYI.")
		return
	}
	// Install docker's GPG key
	resp, err := inet.DoHTTP(inet.HTTPRequest{}, "https://download.docker.com/linux/debian/gpg")
	if err != nil {
		daemon.logPrintStageStep(out, "failed to download docker GPG key - %v", err)
		return
	}
	gpgKeyFile := "/tmp/laitos-docker-gpg-key"
	if err := ioutil.WriteFile(gpgKeyFile, resp.Body, 0600); err != nil {
		daemon.logPrintStageStep(out, "failed to store docker GPG key - %v", err)
		return
	}
	aptOut, err := misc.InvokeProgram(nil, 10, "apt-key", "add", gpgKeyFile)
	daemon.logPrintStageStep(out, "install docker GPG key - %v %s", err, aptOut)
	// Add docker community edition repository
	lsbOut, err := misc.InvokeProgram(nil, 10, "lsb_release", "-cs")
	daemon.logPrintStageStep(out, "determine release name - %v %s", err, lsbOut)
	if err != nil {
		daemon.logPrintStageStep(out, "failed to determine release name")
		return
	}
	aptOut, err = misc.InvokeProgram(nil, 10, "add-apt-repository", fmt.Sprintf("https://download.docker.com/linux/debian %s stable", strings.TrimSpace(string(lsbOut))))
	daemon.logPrintStageStep(out, "enable docker repository - %v %s", err, aptOut)
}

/*
UpgradeInstallSoftware uses Linux package manager to ensure that all system packages are up to date and installs
optional laitos dependencies as well as diagnosis utilities.
*/
func (daemon *Daemon) UpgradeInstallSoftware(out *bytes.Buffer) {
	// Find a system package manager
	var pkgManagerPath, pkgManagerName string
	for _, binPrefix := range []string{"/sbin", "/bin", "/usr/sbin", "/usr/bin", "/usr/sbin/local", "/usr/bin/local"} {
		/*
			Prefer zypper over apt-get bacause opensuse has a weird "apt-get wrapper" that is not remotely functional.
			Prefer apt over apt-get because some public cloud OS templates can upgrade kernel via apt but not with apt-get.
		*/
		for _, execName := range []string{"yum", "zypper", "apt", "apt-get"} {
			pkgManagerPath = filepath.Join(binPrefix, execName)
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
		daemon.logPrintStage(out, "failed to find package manager")
		return
	}
	daemon.logPrintStage(out, "package maintenance via %s", pkgManagerPath)
	// Determine package manager invocation parameters
	var sysUpgradeArgs, installArgs []string
	switch pkgManagerName {
	case "yum":
		// yum is simple and easy
		sysUpgradeArgs = []string{"-y", "-t", "--skip-broken", "update"}
		installArgs = []string{"-y", "-t", "--skip-broken", "install"}
	case "apt":
		// apt and apt-get are too old to be convenient
		fallthrough
	case "apt-get":
		sysUpgradeArgs = []string{"-q", "-y", "-f", "-m", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold", "upgrade"}
		installArgs = []string{"-q", "-y", "-f", "-m", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold", "install"}
	case "zypper":
		// zypper cannot English and integrity
		sysUpgradeArgs = []string{"--non-interactive", "update", "--recommends", "--auto-agree-with-licenses", "--skip-interactive", "--replacefiles", "--force-resolution"}
		installArgs = []string{"--non-interactive", "install", "--recommends", "--auto-agree-with-licenses", "--replacefiles", "--force-resolution"}
	default:
		daemon.logPrintStageStep(out, "failed to find a compatible package manager")
		return
	}
	// apt-get is too old to be convenient, it has to update the manifest first.
	pkgManagerEnv := make([]string, 0, 8)
	if pkgManagerName == "apt-get" {
		pkgManagerEnv = append(pkgManagerEnv, "DEBIAN_FRONTEND=noninteractive")
		// Five minutes should be enough to grab the latest manifest
		result, err := misc.InvokeProgram(pkgManagerEnv, 5*60, pkgManagerPath, "update")
		// There is no need to suppress this output according to markers
		daemon.logPrintStageStep(out, "update apt manifests: %v - %s", err, strings.TrimSpace(result))
	}
	// If package manager output contains any of the strings, the output is reduced into "Nothing to do"
	suppressOutputMarkers := []string{"No packages marked for update", "Nothing to do", "0 upgraded, 0 newly installed", "Unable to locate"}
	// Upgrade system packages with a time constraint of two hours
	result, err := misc.InvokeProgram(pkgManagerEnv, 2*3600, pkgManagerPath, sysUpgradeArgs...)
	for _, marker := range suppressOutputMarkers {
		// If nothing was done during system update, suppress the rather useless output.
		if strings.Contains(result, marker) {
			result = "(nothing to do or upgrade not available)"
			break
		}
	}
	daemon.logPrintStageStep(out, "upgrade system: %v - %s", err, strings.TrimSpace(result))
	/*
		Install additional software packages.
		laitos itself does not rely on any third-party library or program to run, however, it is very useful to install
		several PhantomJS/SlimerJS dependencies, as well as utility applications to help out with system diagnosis.
		Several of the packages are repeated under different names to accommodate the differences in naming convention
		among distributions.
	*/
	pkgs := []string{
		// For outgoing HTTPS connections
		"ca-certificates",

		// Utilities for APT maintenance that also help with installer docker community edition on Debian
		"apt-transport-https", "gnupg", "software-properties-common",
		// Docker for running SlimerJS
		"docker", "docker-client", "docker.io", "docker-ce",

		// Soft and hard dependencies of PhantomJS
		"bzip2", "bzip2-libs", "cjkuni-fonts-common", "cjkuni-ukai-fonts", "cjkuni-uming-fonts", "dbus", "dejavu-fonts-common",
		"dejavu-sans-fonts", "dejavu-serif-fonts", "expat", "firefox", "font-noto", "fontconfig", "fontconfig-config",
		"fontpackages-filesystem", "fonts-arphic-ukai", "fonts-arphic-uming", "fonts-dejavu-core", "fonts-liberation", "freetype",
		"gnutls", "icu", "intlfonts-chinese-big-bitmap-fonts", "intlfonts-chinese-bitmap-fonts", "lib64z1", "libXfont", "libbz2-1",
		"libbz2-1.0", "liberation-fonts-common", "liberation-mono-fonts", "liberation-sans-fonts", "liberation-serif-fonts",
		"liberation2-fonts", "libexpat1", "libfontconfig1", "libfontenc", "libfreetype6", "libicu", "libicu57", "libicu60_2",
		"libpng", "libpng16-16", "nss", "openssl", "ttf-dejavu", "ttf-freefont", "ttf-liberation", "wqy-zenhei", "xorg-x11-font-utils",
		"xorg-x11-fonts-Type1", "zlib", "zlib1g",

		// Time maintenance utilities
		"chrony", "ntp", "ntpd", "ntpdate",
		// Application zip bundle maintenance utilities
		"unzip", "zip",
		// Network diagnosis utilities
		"bind-utils", "curl", "dnsutils", "nc", "net-tools", "netcat", "nmap", "procps", "rsync", "telnet", "tcpdump", "traceroute", "wget", "whois",
		// busybox and toybox are useful for general maintenance, and busybox can synchronise system clock as well.
		"busybox", "toybox",
		// System maintenance utilities
		"lsof", "strace", "sudo", "vim",
	}
	pkgs = append(pkgs, daemon.InstallPackages...)
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
		// Ten minutes should be good enough for each package
		result, err := misc.InvokeProgram(pkgManagerEnv, 10*60, pkgManagerPath, pkgInstallArgs...)
		if err != nil {
			for _, marker := range suppressOutputMarkers {
				// If nothing was done about the package, suppress the rather useless output.
				if strings.Contains(result, marker) {
					result = "(nothing to do or package not available)"
					break
				}
			}
			daemon.logPrintStageStep(out, "install/upgrade %s: %v - %s", name, err, strings.TrimSpace(result))
		}
	}
}

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

// MaintainsIptables blocks ports that are not listed in allowed port and throttle incoming traffic.
func (daemon *Daemon) MaintainsIptables(out *bytes.Buffer) {
	if daemon.BlockPortsExcept == nil || len(daemon.BlockPortsExcept) == 0 {
		return
	}
	daemon.logPrintStage(out, "maintain iptables")
	if daemon.ThrottleIncomingPackets < 5 {
		daemon.logPrintStageStep(out, "ThrottleIncomingPackets(%d) must be greater or equal to 5", daemon.ThrottleIncomingPackets)
		return
	}
	if daemon.ThrottleIncomingPackets > 255 {
		daemon.logPrintStageStep(out, "ThrottleIncomingPackets(%d) must be less than 256, resetting it to 255.", daemon.ThrottleIncomingPackets)
		daemon.ThrottleIncomingPackets = 255
	}
	// Fail safe commands are executed if the usual commands encounter an error. The fail safe permits all traffic.
	failSafe := [][]string{
		{"-F", "OUTPUT"},
		{"-P", "OUTPUT", "ACCEPT"},
		{"-F", "INPUT"},
		{"-P", "INPUT", "ACCEPT"},
	}
	// These are the usual setup commands. Begin by clearing iptables.
	iptables := [][]string{
		{"-F", "OUTPUT"},
		{"-P", "OUTPUT", "ACCEPT"},
		{"-P", "INPUT", "DROP"},
		{"-F", "INPUT"},
	}
	// Work around a redhat kernel bug that prevented throttle counter from exceeding 20
	for _, cmd := range iptables {
		ipOut, ipErr := misc.InvokeProgram(nil, 10, "iptables", cmd...)
		if ipErr != nil {
			daemon.logPrintStageStep(out, "failed in a step that clears iptables - %v - %s", ipErr, ipOut)
		}
	}
	mOut, mErr := misc.InvokeProgram(nil, 10, "modprobe", "-r", "xt_recent")
	daemon.logPrintStageStep(out, "disable xt_recent - %v - %s", mErr, mOut)
	mOut, mErr = misc.InvokeProgram(nil, 10, "modprobe", "xt_recent", "ip_pkt_list_tot=255")
	daemon.logPrintStageStep(out, "re-enable xt_recent - %v - %s", mErr, mOut)

	// After clearing iptables, allow ICMP, established connections, and localhost to communicate
	iptables = append(iptables,
		[]string{"-A", "INPUT", "-p", "icmp", "-j", "ACCEPT"},
		[]string{"-A", "INPUT", "-m", "conntrack", "--ctstate", "INVALID", "-j", "DROP"},
		[]string{"-A", "INPUT", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"},
		[]string{"-A", "INPUT", "-i", "lo", "-j", "ACCEPT"},
		[]string{"-A", "INPUT", "-s", "127.0.0.0/8", "-j", "ACCEPT"},
	)
	// Throttle ports
	for _, port := range daemon.BlockPortsExcept {
		// Throttle new TCP connections
		iptables = append(iptables, []string{"-A", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "-m", "conntrack", "--ctstate", "NEW", "-m", "recent", "--set"})
		iptables = append(iptables, []string{"-A", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "-m", "conntrack", "--ctstate", "NEW", "-m", "recent", "--update", "--seconds", "1", "--hitcount", strconv.Itoa(daemon.ThrottleIncomingPackets), "-j", "DROP"})
		iptables = append(iptables, []string{"-A", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "-m", "conntrack", "--ctstate", "NEW,ESTABLISHED", "-j", "ACCEPT"})

		// Throttle UDP packets
		iptables = append(iptables, []string{"-A", "INPUT", "-p", "udp", "--dport", strconv.Itoa(port), "-m", "recent", "--set"})
		iptables = append(iptables, []string{"-A", "INPUT", "-p", "udp", "--dport", strconv.Itoa(port), "-m", "recent", "--update", "--seconds", "1", "--hitcount", strconv.Itoa(daemon.ThrottleIncomingPackets), "-j", "DROP"})
		iptables = append(iptables, []string{"-A", "INPUT", "-p", "udp", "--dport", strconv.Itoa(port), "-j", "ACCEPT"})
	}
	// Safety rule
	iptables = append(iptables, []string{"-A", "INPUT", "-j", "DROP"})
	// Run setup commands
	for _, args := range iptables {
		ipOut, ipErr := misc.InvokeProgram(nil, 10, "iptables", args...)
		if ipErr != nil {
			daemon.logPrintStageStep(out, "command failed for \"%s\" - %v - %s", strings.Join(args, " "), ipErr, ipOut)
			daemon.logPrintStageStep(out, "configure for fail safe that will allow ALL traffic")
			for _, failSafeCmd := range failSafe {
				failSafeOut, failSafeErr := misc.InvokeProgram(nil, 10, "iptables", failSafeCmd...)
				daemon.logPrintStageStep(out, "fail safe \"%s\" - %v - %s", strings.Join(failSafeCmd, " "), failSafeErr, failSafeOut)
			}
			return
		}
	}
	// Do not touch NAT and Forward as they might have been manipulated by docker daemon
}

// BlockUnusedLogin will block/disable system login from users not listed in the exception list.
func (daemon *Daemon) BlockUnusedLogin(out *bytes.Buffer) {
	if daemon.BlockSystemLoginExcept == nil || len(daemon.BlockSystemLoginExcept) == 0 {
		return
	}
	daemon.logPrintStage(out, "block unused system login")
	exceptionMap := make(map[string]bool)
	for _, name := range daemon.BlockSystemLoginExcept {
		exceptionMap[name] = true
	}
	for userName := range misc.GetLocalUserNames() {
		if exceptionMap[userName] {
			continue
		}
		if ok, blockOut := misc.BlockUserLogin(userName); ok {
			daemon.logPrintStageStep(out, "blocked user %s", userName)
		} else {
			daemon.logPrintStageStep(out, "failed to block user %s - %v", userName, blockOut)
		}
	}
}

// Run unit tests on the maintenance daemon. See TestMaintenance_Execute for daemon setup.
func TestMaintenance(check *Daemon, t testingstub.T) {
	// Make sure maintenance is checking the ports and reporting their errors
	check.CheckTCPPorts = map[string][]int{"localhost": {11334}}
	if result, ok := check.Execute(); ok || !strings.Contains(result, "Port errors") {
		t.Fatal(result)
	}

	// Correct port check errors and continue
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
	// If it fails, the failure could mail processor or HTTP handler
	if result, ok := check.Execute(); !ok &&
		!strings.Contains(result, "Mail processor errors") &&
		!strings.Contains(result, "HTTP handler errors") {
		t.Fatal(result)
	}
	// Break a feature
	check.FeaturesToTest.LookupByTrigger[".s"] = &toolbox.Shell{}
	if result, ok := check.Execute(); ok || !strings.Contains(result, ".s") {
		t.Fatal(result)
	}
	check.FeaturesToTest.LookupByTrigger[".s"] = &toolbox.Shell{InterpreterPath: "/bin/bash"}
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
