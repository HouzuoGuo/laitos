package maintenance

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/httpd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const (
	// TCPPortCheckTimeoutSec is the timeout used in knocking ports.
	TCPPortCheckTimeoutSec = 10
	/*
		MinimumIntervalSec is the lowest acceptable value of system maintenance interval. It must be greater than the
		maximum possible duration of all maintenance tasks together. Be extra careful that Windows system integrity
		maintenance can take couple of hours.
	*/
	MinimumIntervalSec = 24 * 3600
	// InitialDelaySec is the number of seconds to wait for the first maintenance run.
	InitialDelaySec = 180
	// MaxMessageLength is the maximum length of each message entry coming from output of a maintenance action.
	MaxMessageLength = 1024
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
	// EnhanceFileSecurity enables hardening of home directory security (ownership and permission).
	DoEnhanceFileSecurity bool `json:"DoEnhanceFileSecurity"`
	// PreScriptWindows is run by PowerShell prior to all other maintenance actions. It is given 10 minutes to run,
	PreScriptWindows string `json:"PreScriptWindows"`
	// PreScriptWindows is run by Unix default script interpreter prior to all other maintenance actions. It is given 10 minutes to run,
	PreScriptUnix string `json:"PreScriptUnix"`
	/*
		SwapFileSizeMB determines the size of swap file to be created for Linux platform. If the value is 0, no swap file
		will be created; if value is -1, swap will be turned off for the entire OS.
	*/
	SwapFileSizeMB int `json:"SwapFileSizeMB"`
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
	logger        lalog.Logger
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
			if port == 25 && (inet.IsAWS() || inet.IsGCE() || inet.IsAzure() || inet.IsAlibaba()) {
				daemon.logger.Info("runPortsCheck", "", nil, "because Alibaba, Azure, AWS, and Google forbid outgoing connection to port 25, port check will skip %s:25", host)
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
	result.WriteString(common.GetLatestStats())
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
		daemon.IntervalSec = MinimumIntervalSec // quite reasonable to run maintenance daily
	} else if daemon.IntervalSec < MinimumIntervalSec {
		return fmt.Errorf("maintenance.StartAndBlock: IntervalSec must be at or above %d", MinimumIntervalSec)
	}
	daemon.stop = make(chan bool)
	daemon.logger = lalog.Logger{ComponentName: "maintenance", ComponentID: []lalog.LoggerIDField{{Key: "Intv", Value: daemon.IntervalSec}}}
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
	out.WriteString(lalog.TruncateString(fmt.Sprintf("\n---"+template+"\n", a...), MaxMessageLength))
	daemon.logger.Info("maintenance", "", nil, "Stage: "+template, a...)
}

// logPrintStage reports the start/finish of a maintenance step to output buffer and log.
func (daemon *Daemon) logPrintStageStep(out *bytes.Buffer, template string, a ...interface{}) {
	out.WriteString(lalog.TruncateString(fmt.Sprintf("---"+template+"\n", a...), MaxMessageLength))
	daemon.logger.Info("maintenance", "", nil, "Step: "+template, a...)
}

// SystemMaintenance is a long routine that conducts comprehensive general system maintenance tasks.
func (daemon *Daemon) SystemMaintenance() string {
	out := new(bytes.Buffer)
	daemon.logPrintStage(out, "begin system maintenance")

	// In general, an earlier task should exert a positive impact on subsequent tasks.
	daemon.RunPreMaintenanceScript(out)

	// System maintenance
	if daemon.TuneLinux && !misc.HostIsWindows() {
		daemon.logPrintStage(out, "tune linux kernel: %s", toolbox.TuneLinux())
	}
	if daemon.SetTimeZone != "" {
		daemon.logPrintStage(out, "set system time zone to %s", daemon.SetTimeZone)
		if misc.HostIsWindows() {
			daemon.logPrintStage(out, "skipped on windows: set system time zone")
		} else {
			if err := misc.SetTimeZone(daemon.SetTimeZone); err != nil {
				daemon.logPrintStageStep(out, "failed to set time zone: %v", err)
			}
		}
	}
	daemon.CleanUpFiles(out)
	daemon.DefragmentAllDisks(out)
	daemon.TrimAllSSDs(out)
	daemon.MaintainSwapFile(out)

	/*
		It is usually only necessary to copy the utilities once, but two exceptions make it handy to re-copy the
		utilities during maintenance:
		- AWS ElasticBeanstalk the OS template removes unmodified files from /tmp at regular interval.
		- During file system maintenance (step above), unmodified files from 7 days ago are deleted.
		Hence, right here the utility programs are copied once again.
	*/
	daemon.logPrintStage(out, "re-copy non-essential laitos utilities")
	misc.PrepareUtilities(daemon.logger)

	// Software maintenance
	daemon.InstallSoftware(out)
	daemon.MaintainWindowsIntegrity(out)

	// Security maintenance
	daemon.SynchroniseSystemClock(out) // clock synchronisation may depend on a software installed during software maintenance tasks
	daemon.BlockUnusedLogin(out)
	daemon.MaintainServices(out)
	daemon.MaintainsIptables(out) // run this after service maintenance, because disabling firewall service may alter iptables.
	daemon.EnhanceFileSecurity(out)

	daemon.logPrintStage(out, "concluded system maintenance")
	return out.String()
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
	// Should have run pre script as well
	if _, err := os.Stat("/tmp/laitos-maintenance-pre-script-test"); err != nil {
		t.Fatal("did not run pre script")
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
