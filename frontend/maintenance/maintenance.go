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
	"net"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
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
	return fmt.Sprintf(`CmdProc: %s
DNSD TCP/UDP: %s/%s
HTTPD: %s
MAILP: %s
PLAIN TCP/UDP: %s%s
SMTPD: %s
SOCKD TCP/UDP: %s/%s
TELEGRAM BOT: %s
`,
		common.DurationStats.Format(numDecimals),
		dnsd.TCPDurationStats.Format(numDecimals), dnsd.UDPDurationStats.Format(numDecimals),
		api.DurationStats.Format(numDecimals),
		mailp.DurationStats.Format(numDecimals),
		plain.TCPDurationStats.Format(numDecimals), plain.UDPDurationStats.Format(numDecimals),
		smtpd.DurationStats.Format(numDecimals),
		sockd.TCPDurationStats.Format(numDecimals), sockd.UDPDurationStats.Format(numDecimals),
		telegrambot.DurationStats.Format(numDecimals))
}

// Check TCP ports and features, return all-OK or not.
func (check *Maintenance) Execute() (string, bool) {
	check.Logger.Printf("Execute", "", nil, "running now")
	allOK := true
	// Check TCP ports in parallel
	portCheckResult := make(map[int]bool)
	portCheckMutex := new(sync.Mutex)
	waitPorts := new(sync.WaitGroup)
	waitPorts.Add(len(check.TCPPorts))
	for _, portNumber := range check.TCPPorts {
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
	if check.FeaturesToCheck != nil {
		featureErrs = check.FeaturesToCheck.SelfTest()
	}
	var mailpErr error
	if check.MailpToCheck != nil {
		mailpErr = check.MailpToCheck.SelfTest()
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
	for _, portNumber := range check.TCPPorts {
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
		result.WriteString("\nMail processor: OK\n")
	} else {
		result.WriteString(fmt.Sprintf("\nMail processor: %v\n", mailpErr))
	}
	// Warnings, logs, and stack traces
	result.WriteString("\nWarnings:\n")
	result.WriteString(feature.GetLatestWarnings())
	result.WriteString("\nLogs:\n")
	result.WriteString(feature.GetLatestLog())
	result.WriteString("\nStack traces:\n")
	result.WriteString(feature.GetGoroutineStacktraces())
	// Send away!
	if allOK {
		check.Logger.Printf("Execute", "", nil, "completed with everything being OK")
	} else {
		check.Logger.Warningf("Execute", "", nil, "completed with some errors")
	}
	if err := check.Mailer.Send(email.OutgoingMailSubjectKeyword+"-maintenance", result.String(), check.Recipients...); err != nil {
		check.Logger.Warningf("Execute", "", err, "failed to send notification mail")
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

func (check *Maintenance) Initialise() error {
	check.Logger = global.Logger{ComponentName: "Maintenance", ComponentID: strconv.Itoa(check.IntervalSec)}
	if check.IntervalSec < 120 {
		return errors.New("Maintenance.StartAndBlock: IntervalSec must be above 119")
	}
	check.stop = make(chan bool)
	return nil
}

/*
You may call this function only after having called Initialise()!
Start health check loop and block caller until Stop function is called.
*/
func (check *Maintenance) StartAndBlock() error {
	sort.Ints(check.TCPPorts)
	for {
		if global.EmergencyLockDown {
			return global.ErrEmergencyLockDown
		}
		atomic.StoreInt32(&check.loopIsRunning, 1)
		select {
		case <-check.stop:
			return nil
		case <-time.After(time.Duration(check.IntervalSec) * time.Second):
			check.Execute()
		}
	}
}

// Stop previously started health check loop.
func (check *Maintenance) Stop() {
	if atomic.CompareAndSwapInt32(&check.loopIsRunning, 1, 0) {
		check.stop <- true
	}
}

/*
UpgradeSystem uses Linux package manager to ensure that all system packages are up to date and laitos dependencies
are installed and up to date. Returns human-readable result output.
*/
func (check *Maintenance) UpgradeSystem() string {
	/*
		Debian and derivatives:
		# apt-get update
		# DEBIAN_FRONTEND=noninteractive apt-get -q -y -f -m -o "Dpkg::Options::=--force-confdef" -o "Dpkg::Options::=--force-confold" upgrade
		# DEBIAN_FRONTEND=noninteractive apt-get -q -y -f -m -o "Dpkg::Options::=--force-confdef" -o "Dpkg::Options::=--force-confold" install
		AWS Linux, Centos, and more:
		# yum -y -t --skip-broken update
		# yum -y -t --skip-broken install
		openSUSE:
		# zypper --non-interactive update --recommends --auto-agree-with-licenses --skip-interactive --replacefiles --force-resolution
		# zypper --non-interactive install --recommends --auto-agree-with-licenses --replacefiles --force-resolution

		Packages:
		zlib  zlib1g lib64z1
		fontconfig libfontconfig1
		libfreetype6 freetype
		libexpat1 expat
		libbz2-1 bzip2-libs libbz2-1.0
		libpng16-16 libpng libpng16-16
		busybox
	*/
	ret := new(bytes.Buffer)
	/*
		laitos itself does not rely on any third-party library or program to run, however, the PhantomJS component requires
		these packages to run. Busybox is not required by PhantomJS, but it is included just for fun.
		Some of the packages are repeated under different names to accommodate the differences in naming convention among distributions.
	*/
	//pkgs := []string{"busybox", "bzip2-libs", "expat", "fontconfig", "freetype", "lib64z1", "libbz2-1", "libbz2-1.0", "libexpat1", "libfontconfig1", "libfreetype6", "libpng", "libpng16-16", "zlib", "zlib1g"}
	// Find a system package manager
	var pkgManagerPath, pkgManagerName string
	for _, binPrefix := range []string{"/sbin", "/bin", "/usr/sbin", "/usr/bin", "/usr/sbin/local", "/usr/bin/local"} {
		for _, execName := range []string{"yum", "apt-get", "zypper"} {
			pkgManagerPath = path.Join(binPrefix, execName)
			pkgManagerName = execName
			break
		}
		if pkgManagerName != "" {
			break
		}
	}
	// apt-get is too old to be convenient
	var aptEnvVars []string
	if pkgManagerName == "apt-get" {
		ret.WriteString("Calling apt-get update...\n")
		aptEnvVars = []string{"DEBIAN_FRONTEND=noninteractive"}
		result, err := env.InvokeProgram(aptEnvVars, 180, pkgManagerPath, "update")
		fmt.Fprintf(ret, "apt-get update result: %s\napt-get update error: %v\n", result, err)
	}
	return ret.String()
}

// Run unit tests on the health checker. See TestMaintenance_Execute for daemon setup.
func TestMaintenance(check *Maintenance, t *testing.T) {
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
