package maintenance

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/env"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/frontend/dnsd"
	"github.com/HouzuoGuo/laitos/frontend/httpd/api"
	"github.com/HouzuoGuo/laitos/frontend/mailp"
	"github.com/HouzuoGuo/laitos/frontend/plain"
	"github.com/HouzuoGuo/laitos/frontend/smtpd"
	"github.com/HouzuoGuo/laitos/frontend/sockd"
	"github.com/HouzuoGuo/laitos/frontend/telegrambot"
	"github.com/HouzuoGuo/laitos/global"
	"github.com/HouzuoGuo/laitos/testingstub"
	"net"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
)

const (
	TCPConnectionTimeoutSec = 10
)

/*
Maintenance is a daemon that triggers health check and system maintenance periodically. Health check comprises port
checks, API key checks, and a lot more. System maintenance ensures that system packages are up to date and dependencies
of this program are installed and up to date.
The result of each run is is sent to designated email addresses, along with latest environment information such as
latest logs and warnings.
*/
type Maintenance struct {
	TCPPorts        []int                `json:"TCPPorts"`    // Check that these TCP ports are listening on this host
	IntervalSec     int                  `json:"IntervalSec"` // Check TCP ports and features at this interval
	Mailer          email.Mailer         `json:"Mailer"`      // Send notification mails via this mailer
	Recipients      []string             `json:"Recipients"`  // Address of recipients of notification mails
	FeaturesToCheck *feature.FeatureSet  `json:"-"`           // Health check subject - features and their API keys
	MailpToCheck    *mailp.MailProcessor `json:"-"`           // Health check subject - mail processor and its mailer
	Logger          global.Logger        `json:"-"`           // Logger
	loopIsRunning   int32                // Value is 1 only when health check loop is running
	stop            chan bool            // Signal health check loop to stop
}

/*
GetLatestStats returns statistic information from all front-end daemons, each on their own line.
Due to inevitable cyclic import, this function is defined twice, once in api.go of api package, the other in
maintenance.go of maintenance package.
*/
func GetLatestStats() string {
	numDecimals := 2
	factor := 1000000000.0
	return fmt.Sprintf(`CmdProc: %s
DNSD TCP/UDP: %s/%s
HTTPD: %s
MAILP: %s
PLAIN TCP/UDP: %s%s
SMTPD: %s
SOCKD TCP/UDP: %s/%s
TELEGRAM BOT: %s
`,
		common.DurationStats.Format(factor, numDecimals),
		dnsd.TCPDurationStats.Format(factor, numDecimals), dnsd.UDPDurationStats.Format(factor, numDecimals),
		api.DurationStats.Format(factor, numDecimals),
		mailp.DurationStats.Format(factor, numDecimals),
		plain.TCPDurationStats.Format(factor, numDecimals), plain.UDPDurationStats.Format(factor, numDecimals),
		smtpd.DurationStats.Format(factor, numDecimals),
		sockd.TCPDurationStats.Format(factor, numDecimals), sockd.UDPDurationStats.Format(factor, numDecimals),
		telegrambot.DurationStats.Format(factor, numDecimals))
}

// Check TCP ports and features, return all-OK or not.
func (maint *Maintenance) Execute() (string, bool) {
	maint.Logger.Printf("Execute", "", nil, "running now")
	// Conduct system maintenance first to ensure an accurate reading of runtime information later on
	maintResult := maint.SystemMaintenance()
	// allOK is true only if all port and feature checks pass, system maintenance always succeeds.
	allOK := true
	// Check TCP ports in parallel
	portCheckResult := make(map[int]bool)
	portCheckMutex := new(sync.Mutex)
	waitPorts := new(sync.WaitGroup)
	waitPorts.Add(len(maint.TCPPorts))
	for _, portNumber := range maint.TCPPorts {
		go func(portNumber int) {
			conn, err := net.DialTimeout("tcp", "localhost:"+strconv.Itoa(portNumber), TCPConnectionTimeoutSec*time.Second)
			portCheckMutex.Lock()
			portCheckResult[portNumber] = err == nil
			allOK = allOK && portCheckResult[portNumber]
			portCheckMutex.Unlock()
			if err == nil {
				conn.Close()
			}
			waitPorts.Done()
		}(portNumber)
	}
	waitPorts.Wait()
	// Check features and mail processor
	featureErrs := make(map[feature.Trigger]error)
	if maint.FeaturesToCheck != nil {
		featureErrs = maint.FeaturesToCheck.SelfTest()
	}
	var mailpErr error
	if maint.MailpToCheck != nil {
		mailpErr = maint.MailpToCheck.SelfTest()
	}
	allOK = allOK && len(featureErrs) == 0 && mailpErr == nil
	// Compose mail body
	var result bytes.Buffer
	if allOK {
		result.WriteString("All OK\n")
	} else {
		result.WriteString("There are errors!!!\n")
	}
	// Runtime info
	result.WriteString(feature.GetRuntimeInfo())
	// Statistics
	result.WriteString("\nStatistics (ms):\n")
	result.WriteString(GetLatestStats())
	// Port checks
	result.WriteString("\nPorts:\n")
	for _, portNumber := range maint.TCPPorts {
		if portCheckResult[portNumber] {
			result.WriteString(fmt.Sprintf("%d-OK ", portNumber))
		} else {
			result.WriteString(fmt.Sprintf("%d-Error ", portNumber))
		}
	}
	result.WriteRune('\n')
	// Feature checks
	if len(featureErrs) == 0 {
		result.WriteString("\nFeatures: OK\n")
	} else {
		for trigger, err := range featureErrs {
			result.WriteString(fmt.Sprintf("\nFeatures %s: %+v\n", trigger, err))
		}
	}
	// Mail processor checks
	if mailpErr == nil {
		result.WriteString("Mail processor: OK\n")
	} else {
		result.WriteString(fmt.Sprintf("Mail processor: %v\n", mailpErr))
	}
	// Maintenance results, warnings, logs, and stack traces, in that order.
	result.WriteString("\nSystem maintenance:\n")
	result.WriteString(maintResult)
	result.WriteString("\nWarnings:\n")
	result.WriteString(feature.GetLatestWarnings())
	result.WriteString("\nLogs:\n")
	result.WriteString(feature.GetLatestLog())
	result.WriteString("\nStack traces:\n")
	result.WriteString(feature.GetGoroutineStacktraces())
	// Send away!
	if allOK {
		maint.Logger.Printf("Execute", "", nil, "completed with everything being OK")
	} else {
		maint.Logger.Warningf("Execute", "", nil, "completed with some errors")
	}
	if err := maint.Mailer.Send(email.OutgoingMailSubjectKeyword+"-maintenance", result.String(), maint.Recipients...); err != nil {
		maint.Logger.Warningf("Execute", "", err, "failed to send notification mail")
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

func (maint *Maintenance) Initialise() error {
	maint.Logger = global.Logger{ComponentName: "Maintenance", ComponentID: strconv.Itoa(maint.IntervalSec)}
	if maint.IntervalSec < 120 {
		return errors.New("Maintenance.StartAndBlock: IntervalSec must be above 119")
	}
	maint.stop = make(chan bool)
	return nil
}

/*
You may call this function only after having called Initialise()!
Start health check loop and block caller until Stop function is called.
*/
func (maint *Maintenance) StartAndBlock() error {
	// Sort port numbers so that their check results look nicer in the final report
	sort.Ints(maint.TCPPorts)
	firstTime := true
	for {
		if global.EmergencyLockDown {
			return global.ErrEmergencyLockDown
		}
		// The very first health check is executed soon (10 minutes) after health check daemon starts up
		var waitDuration time.Duration
		if firstTime {
			waitDuration = 10 * time.Minute
			firstTime = false
		} else {
			waitDuration = time.Duration(maint.IntervalSec) * time.Second
		}
		atomic.StoreInt32(&maint.loopIsRunning, 1)
		select {
		case <-maint.stop:
			return nil
		case <-time.After(waitDuration):
			maint.Execute()
		}
	}
}

// Stop previously started health check loop.
func (maint *Maintenance) Stop() {
	if atomic.CompareAndSwapInt32(&maint.loopIsRunning, 1, 0) {
		maint.stop <- true
	}
}

/*
SystemMaintenance uses Linux package manager to ensure that all system packages are up to date and laitos dependencies
are installed and up to date. Returns human-readable result output.
*/
func (maint *Maintenance) SystemMaintenance() string {
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
		maint.Logger.Printf("SystemMaintenance", "", nil, "updating apt manifests")
		result, err := env.InvokeProgram(pkgManagerEnv, 5*60, pkgManagerPath, "update")
		maint.Logger.Printf("SystemMaintenance", "", err, "finished updating apt manifests")
		fmt.Fprintf(ret, "--- apt-get result: %v - %s\n\n", err, result)
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
	// Upgrade system packages with a time constraint of two hours
	ret.WriteString("--- Upgrading system packages...\n")
	maint.Logger.Printf("SystemMaintenance", "", nil, "updating system packages")
	result, err := env.InvokeProgram(pkgManagerEnv, 2*3600, pkgManagerPath, sysUpgradeArgs...)
	maint.Logger.Printf("SystemMaintenance", "", err, "finished updating system packages")
	fmt.Fprintf(ret, "--- System upgrade result: %v - %s\n\n", err, result)
	/*
		Install additional software packages.
		laitos itself does not rely on any third-party library or program to run, however, the PhantomJS component requires
		these packages to run. Busybox is not required by PhantomJS, but it is included just for fun.
		Zip and unzip are useful for maintaining application bundles.
		Some of the packages are repeated under different names to accommodate the differences in naming convention among distributions.
	*/
	pkgs := []string{
		"busybox", "bzip2-libs", "expat", "fontconfig", "freetype",
		"lib64z1", "libbz2-1", "libbz2-1.0", "libexpat1", "libfontconfig1", "libfreetype6", "libpng", "libpng16-16",
		"unzip", "zip", "zlib", "zlib1g"}
	/*
		Although all three package managers can install more than one packages at a time, the packages are still
		installed one after another, because:
		- apt-get does not ignore non-existent package names, how inconvenient.
		- if zypper runs into unsatisfactory package dependencies, it aborts the whole installation.
		yum is once again the easiest one to work with.
	*/
	for _, name := range pkgs {
		// Put software name next to installation parameters
		pkgInstallArgs := make([]string, len(installArgs)+1)
		copy(pkgInstallArgs, installArgs)
		pkgInstallArgs[len(installArgs)] = name
		fmt.Fprintf(ret, "--- Installing/upgrading %s\n", name)
		// Five minutes should be good enough for every package
		maint.Logger.Printf("SystemMaintenance", "", nil, "installing package %s", name)
		result, err := env.InvokeProgram(pkgManagerEnv, 5*60, pkgManagerPath, pkgInstallArgs...)
		maint.Logger.Printf("SystemMaintenance", "", err, "finished installing package %s", name)
		fmt.Fprintf(ret, "--- %s installation/upgrade result: %v - %s\n\n", name, err, result)
	}
	ret.WriteString("--- System maintenance has finished.\n")
	maint.Logger.Printf("SystemMaintenance", "", nil, "all finished")
	return ret.String()
}

// Run unit tests on the health checker. See TestMaintenance_Execute for daemon setup.
func TestMaintenance(check *Maintenance, t testingstub.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	go func() {
		if err != nil {
			t.Fatal(err)
		}
		// Accept exactly one connection that is from health checker
		listener.Accept()
	}()
	// Port should be now listening
	time.Sleep(1 * time.Second)
	check.TCPPorts = []int{listener.Addr().(*net.TCPAddr).Port}
	// If it fails, the failure could only come from mailer of mail processor.
	if result, ok := check.Execute(); !ok && !strings.Contains(result, "Mailer.SelfTest") {
		t.Fatal(result)
	}
	// Break a feature
	check.FeaturesToCheck.LookupByTrigger[".s"] = &feature.Shell{}
	if result, ok := check.Execute(); ok || !strings.Contains(result, ".s") {
		t.Fatal(result)
	}
	check.FeaturesToCheck.LookupByTrigger[".s"] = &feature.Shell{InterpreterPath: "/bin/bash"}
	// Expect checks to begin within a second
	if err := check.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Health check should successfully start within two seconds
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
