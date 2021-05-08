package maintenance

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/awsinteg"
	"github.com/HouzuoGuo/laitos/daemon/httpd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
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
	// PrometheusProcessMetricsInterval is the interval at which the latest process performance measurements are
	// collected and then given to prometheus metrics.
	PrometheusProcessMetricsInterval = 10 * time.Second
)

// ReportFilePath is the absolute file path to the text report from latest maintenance run.
var ReportFilePath = path.Join(os.TempDir(), "laitos-latest-maintenance-report.txt")

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
	// ProvidePerformanceMetricsToPrometheus determines whether the maintenance daemon will provide program performance metrics to prometheus at regular interval.
	RegisterPrometheusMetrics bool `json:"RegisterPrometheusMetrics"`

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

	// UploadReportToS3Bucket is the name of S3 bucket into which the maintenance daemon shall upload its summary reports.
	UploadReportToS3Bucket string `json:"UploadReportToS3Bucket"`

	processExplorerMetrics *ProcessExplorerMetrics
	lastStepTimestamp      int64 // lastStepTimestamp is the unix timestamp at which the last maintenance stage or a stage stap took place
	runContext             context.Context
	runCancelFunc          context.CancelFunc
	logger                 lalog.Logger
}

// runPortsCheck knocks on TCP ports that are to be checked in parallel, it returns an error if any of the ports fails to connect.
func (daemon *Daemon) runPortsCheck() error {
	if daemon.CheckTCPPorts == nil {
		return nil
	}

	portErrs := make([]string, 0)
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
					daemon.logger.MaybeMinorError(conn.Close())
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
	waitAllChecks.Add(4) // will wait for port checks, app tests, mail command runner, and HTTP handler tests.
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
	summary := platform.GetProgramStatusSummary(true)
	result.WriteString(summary.String())
	result.WriteString("\nDaemon stats - low/avg/high/total seconds and (count):\n")
	result.WriteString(misc.GetLatestStats())
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
	// If there are no recipients, print the report to standard output.
	if daemon.Recipients == nil || len(daemon.Recipients) == 0 {
		daemon.logger.Info("Execute", "", nil, "report will now be printed to standard output")
		fmt.Println("Maintenance report:")
		fmt.Println(result.String())
	} else if err := daemon.MailClient.Send(inet.OutgoingMailSubjectKeyword+"-maintenance", result.String(), daemon.Recipients...); err != nil {
		daemon.logger.Warning("Execute", "", err, "failed to send notification mail")
	}
	// Leave the latest maintenance report in system temporary directory for inspection, overwrite existing report if there is any.
	if err := ioutil.WriteFile(ReportFilePath, result.Bytes(), 0600); err != nil {
		daemon.logger.Warning("Execute", "", err, "failed to persist latest maintenance report in %s, you may still find the report in Email or laitos program output.", ReportFilePath)
	}
	if misc.EnableAWSIntegration {
		// Upload the latest maintenance report to S3 bucket, named the object after the date and time of the system wall clock.
		go func() {
			daemon.logger.Info("Execute", "", nil, "will store a copy of the report in S3 bucket %s", daemon.UploadReportToS3Bucket)
			s3Client, err := awsinteg.NewS3Client()
			if err != nil {
				daemon.logger.Warning("Execute", daemon.UploadReportToS3Bucket, err, "failed to initialise S3 client")
				return
			}
			// Spend at most 60 seconds at uploading the report file
			uploadTimeoutCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			_ = s3Client.Upload(uploadTimeoutCtx, daemon.UploadReportToS3Bucket, time.Now().Format(time.RFC3339), bytes.NewReader(result.Bytes()))
		}()
	}
	return lalog.LintString(result.String(), inet.MaxMailBodySize), allOK
}

func (daemon *Daemon) Initialise() error {
	if daemon.IntervalSec < 1 {
		daemon.IntervalSec = MinimumIntervalSec // quite reasonable to run maintenance daily
	} else if daemon.IntervalSec < MinimumIntervalSec {
		return fmt.Errorf("maintenance.Initialise: IntervalSec must be at or above %d", MinimumIntervalSec)
	}
	daemon.logger = lalog.Logger{ComponentName: "maintenance", ComponentID: []lalog.LoggerIDField{{Key: "Intv", Value: daemon.IntervalSec}}}
	if daemon.RegisterPrometheusMetrics && misc.EnablePrometheusIntegration {
		daemon.processExplorerMetrics = NewProcessExplorerMetrics()
		if err := daemon.processExplorerMetrics.RegisterGlobally(); err != nil {
			daemon.logger.Warning("Initialise", "prometheus", err, "failed to register metrics with prometheus")
		}
	}
	return nil
}

/*
You may call this function only after having called Initialise()!
Start health check loop and block caller until Stop function is called.
*/
func (daemon *Daemon) StartAndBlock() error {
	daemon.runContext, daemon.runCancelFunc = context.WithCancel(context.Background())
	maintenanceRoutineTicker := time.NewTicker(time.Duration(daemon.IntervalSec) * time.Second)
	processMetricsRefreshTicker := time.NewTicker(PrometheusProcessMetricsInterval)
	defer func() {
		processMetricsRefreshTicker.Stop()
		maintenanceRoutineTicker.Stop()
	}()
	// Run maintenance routine at regular interval
	go func() {
		firstRunDelay := time.After(2 * time.Minute)
		daemon.logger.Info("StartAndBlock", "", nil, "the maintenance routines will run in 2 minutes, and then every ~%d hours", daemon.IntervalSec/3600)
		for {
			if misc.EmergencyLockDown {
				return
			}
			select {
			case <-daemon.runContext.Done():
				return
			case <-firstRunDelay:
				// The first run does not have to wait for the interval to pass
				daemon.Execute()
			case <-maintenanceRoutineTicker.C:
				daemon.Execute()
			}
		}
	}()
	// Collect latest performance measurements at regular interval
	if daemon.processExplorerMetrics != nil {
		daemon.logger.Info("StartAndBlock", "prometheus", nil, "will regularly take program performance measurements and give them to prometheus metrics.")
		go func() {
			for {
				if misc.EmergencyLockDown {
					return
				}
				select {
				case <-daemon.runContext.Done():
					return
				case <-processMetricsRefreshTicker.C:
					if daemon.processExplorerMetrics != nil {
						if err := daemon.processExplorerMetrics.Refresh(); err != nil {
							daemon.logger.Warning("StartAndBlock", "prometheus", err, "failed to collect the latest process performance measurements")
						}
					}
				}
			}
		}()
	}
	// Wait for daemon to stop
	<-daemon.runContext.Done()
	daemon.logger.Info("StartAndBlock", "", nil, "stopped on request")
	return nil
}

// Stop the daemon.
func (daemon *Daemon) Stop() {
	daemon.runCancelFunc()
}

// logPrintStage reports the start/finish of a maintenance stage to the output buffer and program log.
func (daemon *Daemon) logPrintStage(out *bytes.Buffer, template string, a ...interface{}) {
	if duration := time.Now().Unix() - daemon.lastStepTimestamp; duration > 5 {
		out.WriteString(fmt.Sprintf("(it took %d seconds)\n", duration))
	}
	out.WriteString(lalog.TruncateString(fmt.Sprintf("\n---"+template+"\n", a...), MaxMessageLength))
	daemon.logger.Info("maintenance", "", nil, "Stage: "+template, a...)
	daemon.lastStepTimestamp = time.Now().Unix()
}

// logPrintStage reports the start/finish of a maintenance step to the output buffer and program log.
func (daemon *Daemon) logPrintStageStep(out *bytes.Buffer, template string, a ...interface{}) {
	if duration := time.Now().Unix() - daemon.lastStepTimestamp; duration > 5 {
		out.WriteString(fmt.Sprintf("(it took %d seconds)\n", duration))
	}
	out.WriteString(lalog.TruncateString(fmt.Sprintf("---"+template+"\n", a...), MaxMessageLength))
	daemon.logger.Info("maintenance", "", nil, "Step: "+template, a...)
	daemon.lastStepTimestamp = time.Now().Unix()
}

// SystemMaintenance is a long routine that conducts comprehensive general system maintenance tasks.
func (daemon *Daemon) SystemMaintenance() string {
	out := new(bytes.Buffer)
	daemon.logPrintStage(out, "begin system maintenance")

	// In general, an earlier task should exert a positive impact on subsequent tasks.
	daemon.RunPreMaintenanceScript(out)

	// System maintenance
	if daemon.TuneLinux && !platform.HostIsWindows() {
		daemon.logPrintStage(out, "tune linux kernel: %s", toolbox.TuneLinux())
	}
	if daemon.SetTimeZone != "" {
		daemon.logPrintStage(out, "set system time zone to %s", daemon.SetTimeZone)
		if platform.HostIsWindows() {
			daemon.logPrintStage(out, "skipped on windows: set system time zone")
		} else {
			if err := platform.SetTimeZone(daemon.SetTimeZone); err != nil {
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
	platform.CopyNonEssentialUtilities(daemon.logger)

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
	defer os.RemoveAll(ReportFilePath)
	os.Remove(ReportFilePath)
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
	if result, ok := check.Execute(); ok || !strings.Contains(result, "Shell.SelfTest") { // broken shell configuration
		t.Fatal(result)
	}
	// Look for maintenance report in temporary file
	if content, err := ioutil.ReadFile(ReportFilePath); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(string(content), "Shell.SelfTest") { // broken shell configuration
		t.Fatal(string(content))
	}
	check.FeaturesToTest.LookupByTrigger[".s"] = &toolbox.Shell{InterpreterPath: "/bin/bash"}
	// Expect checks to begin within a second
	if err := check.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Maintenance loop should successfully start within two seconds
	serverStopped := make(chan struct{}, 1)
	go func() {
		if err := check.StartAndBlock(); err != nil {
			t.Error(err)
			return
		}
		serverStopped <- struct{}{}
	}()
	time.Sleep(2 * time.Second)

	check.Stop()
	<-serverStopped
	// Repeatedly stopping the daemon should have no negative consequence
	check.Stop()
	check.Stop()
}
