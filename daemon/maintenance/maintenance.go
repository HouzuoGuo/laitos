package maintenance

import (
	"bytes"
	"context"
	"fmt"
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
	// ScriptForWindows is a PowerShell Script to be run on Windows at the end of maintenance procedure.
	ScriptForWindows string `json:"ScriptForWindows"`
	// ScriptForUnix is a shell script to be run on Unix/Linux at the end of maintenance procedure.
	ScriptForUnix string `json:"ScriptForUnix"`
	// SwapFileSizeMB is the size of swap file to be created and activated for a Linux host.
	// If the value is 0 then no swap file will be created.
	// If the value is -1 then all active swap files and swap partitions will be disabled.
	SwapFileSizeMB int `json:"SwapFileSizeMB"`
	// SetTimeZone changes system time zone to the specified value (such as "UTC" or "Europe/Dublin").
	SetTimeZone string `json:"SetTimeZone"`
	// RegisterPrometheusMetrics records process statistics (e.g. CPU time & context switches) in promehteus metrics.
	RegisterPrometheusMetrics bool `json:"RegisterPrometheusMetrics"`
	// RegsiterProcessActivityMetrics records process file and network activities powered by eBPF in prometheus metrics.
	RegsiterProcessActivityMetrics bool `json:"RegsiterProcessActivityMetrics"`
	// RegsiterProcessActivityMetrics records system file and network activities powered by eBPF in prometheus metrics.
	// This is expensive and requires an additional 500MB of memory.
	RegisterSystemActivityMetrics bool `json:"RegsiterSystemActivityMetrics"`
	// PrometheusScrapIntervalSec is the scrape interval from prometheus. The interval helps determine the sampling period of certain gauges.
	PrometheusScrapeIntervalSec int `json:"PrometheusScrapeIntervalSec"`
	// ShrinkSystemdJournalSizeMB is the threshold under which systemd journal will be shrunk. Older journal will be deleted.
	ShrinkSystemdJournalSizeMB int `json:"ShrinkSystemdJournalSizeMB"`

	/*
		IntervalSec determines the rate of execution of maintenance routine. This is not a sleep duration. The constant
		rate of execution is maintained by taking away routine's elapsed time from actual interval between runs.
	*/
	IntervalSec               int                     `json:"IntervalSec"`
	MailClient                inet.MailClient         `json:"MailClient"` // Send notification mails via this mailer
	Recipients                []string                `json:"Recipients"` // Address of recipients of notification mails
	ToolboxSelfTest           *toolbox.FeatureSet     `json:"-"`          // FeaturesToTest are toolbox features to be tested during health check.
	MailCommandRunnerSelfTest *mailcmd.CommandRunner  `json:"-"`          // MailCmdRunnerToTest is mail command runner to be tested during health check.
	HttpHandlersSelfTest      httpd.HandlerCollection `json:"-"`          // HTTPHandlersToCheck are the URL handlers of an HTTP daemon to be tested during health check.

	// UploadReportToS3Bucket is the name of S3 bucket into which the maintenance daemon shall upload its summary reports.
	UploadReportToS3Bucket string `json:"UploadReportToS3Bucket"`

	lastStepTimestamp      int64 // lastStepTimestamp is the unix timestamp at which the last maintenance stage or a stage step took place
	processExplorerMetrics *ProcessExplorerMetrics

	cancelFunc context.CancelFunc
	logger     *lalog.Logger
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
				daemon.logger.Info("", nil, "because Alibaba, Azure, AWS, and Google forbid outgoing connection to port 25, port check will skip %s:25", host)
				continue
			}
			wait.Add(1)
			go func(host string, port int) {
				defer wait.Done()
				if !misc.ProbePort(15*time.Second, host, port) {
					portErrsMutex.Lock()
					portErrs = append(portErrs, net.JoinHostPort(host, strconv.Itoa(port)))
					portErrsMutex.Unlock()
				}
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
func (daemon *Daemon) Execute(ctx context.Context) (string, bool) {
	daemon.logger.Info("", nil, "running now")
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
		if daemon.ToolboxSelfTest != nil {
			featureErr = daemon.ToolboxSelfTest.SelfTest()
		}
		waitAllChecks.Done()
	}()
	go func() {
		// Mail command runner test - the routine itself also uses concurrency internally
		if daemon.MailCommandRunnerSelfTest != nil && daemon.MailCommandRunnerSelfTest.ReplyMailClient.IsConfigured() {
			mailCmdRunnerErr = daemon.MailCommandRunnerSelfTest.SelfTest()
		}
		waitAllChecks.Done()
	}()
	go func() {
		// HTTP special handler test - the routine itself also uses concurrency internally
		if daemon.HttpHandlersSelfTest != nil {
			httpHandlersErr = daemon.HttpHandlersSelfTest.SelfTest()
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
		result.WriteString("\nApp toolbox: OK\n")
	} else {
		result.WriteString(fmt.Sprintf("\nApp toolbox errors: %v\n", featureErr))
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
		daemon.logger.Info("", nil, "completed with everything being OK")
	} else {
		daemon.logger.Warning("", nil, "completed with some errors")
	}
	// If there are no recipients, print the report to standard output.
	if len(daemon.Recipients) == 0 {
		daemon.logger.Info("", nil, "report will now be printed to standard output")
		fmt.Println("Maintenance report:")
		fmt.Println(result.String())
	} else if err := daemon.MailClient.Send(inet.OutgoingMailSubjectKeyword+"-maintenance", result.String(), daemon.Recipients...); err != nil {
		daemon.logger.Warning("", err, "failed to send notification mail")
	}
	// Leave the latest maintenance report in system temporary directory for inspection, overwrite existing report if there is any.
	if err := os.WriteFile(ReportFilePath, result.Bytes(), 0600); err != nil {
		daemon.logger.Warning("", err, "failed to persist latest maintenance report in %s, you may still find the report in Email or laitos program output.", ReportFilePath)
	}
	if misc.EnableAWSIntegration {
		// Upload the latest maintenance report to S3 bucket, named the object after the date and time of the system wall clock.
		go func() {
			daemon.logger.Info("", nil, "will store a copy of the report in S3 bucket %s", daemon.UploadReportToS3Bucket)
			s3Client, err := awsinteg.NewS3Client()
			if err != nil {
				daemon.logger.Warning(daemon.UploadReportToS3Bucket, err, "failed to initialise S3 client")
				return
			}
			// Spend at most 60 seconds at uploading the report file
			uploadTimeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
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
	if daemon.PrometheusScrapeIntervalSec < 1 {
		daemon.PrometheusScrapeIntervalSec = 60
	}
	daemon.logger = &lalog.Logger{ComponentName: "maintenance", ComponentID: []lalog.LoggerIDField{{Key: "Intv", Value: daemon.IntervalSec}}}
	if daemon.RegisterPrometheusMetrics && misc.EnablePrometheusIntegration {
		daemon.processExplorerMetrics = NewProcessExplorerMetrics(lalog.DefaultLogger, daemon.PrometheusScrapeIntervalSec, daemon.RegsiterProcessActivityMetrics, daemon.RegisterSystemActivityMetrics)
		if err := daemon.processExplorerMetrics.RegisterGlobally(); err != nil {
			daemon.logger.Warning("prometheus", err, "failed to register metrics with prometheus")
		}
	}
	return nil
}

/*
You may call this function only after having called Initialise()!
Start health check loop and block caller until Stop function is called.
*/
func (daemon *Daemon) StartAndBlock() error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	daemon.cancelFunc = cancelFunc

	// Run maintenance routine at regular interval
	periodicMaint := &misc.Periodic{
		LogActorName:   daemon.logger.ComponentName,
		Interval:       time.Duration(daemon.IntervalSec) * time.Second,
		StableInterval: true,
		MaxInt:         1,
		Func: func(ctx context.Context, round, _ int) error {
			if round == 0 {
				daemon.logger.Info("", nil, "the first run will begin in about two minutes")
				select {
				case <-time.After(2 * time.Minute):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			daemon.Execute(ctx)
			return nil
		},
	}
	if err := periodicMaint.Start(ctx); err != nil {
		return err
	}

	// Collect latest performance measurements at regular interval
	if daemon.processExplorerMetrics != nil {
		daemon.logger.Info("", nil, "will regularly take program performance measurements and give them to prometheus metrics.")
		periodicProcMetrics := &misc.Periodic{
			LogActorName: "refresh-process-explorer-metrics",
			Interval:     time.Duration(daemon.PrometheusScrapeIntervalSec) * time.Second,
			MaxInt:       1,
			Func: func(context.Context, int, int) error {
				if daemon.processExplorerMetrics != nil {
					if err := daemon.processExplorerMetrics.Refresh(); err != nil {
						daemon.logger.Warning("prometheus", err, "failed to collect the latest process performance measurements")
					}
				}
				return nil
			},
		}
		if err := periodicProcMetrics.Start(ctx); err != nil {
			return err
		}
	}
	return periodicMaint.WaitForErr()
}

// Stop the daemon.
func (daemon *Daemon) Stop() {
	daemon.cancelFunc()
}

// logPrintStage reports the start/finish of a maintenance stage to the output buffer and program log.
func (daemon *Daemon) logPrintStage(out *bytes.Buffer, template string, a ...interface{}) {
	if duration := time.Now().Unix() - daemon.lastStepTimestamp; duration > 5 {
		out.WriteString(fmt.Sprintf("(it took %d seconds)\n", duration))
	}
	out.WriteString(lalog.TruncateString(fmt.Sprintf("\n---"+template+"\n", a...), lalog.MaxLogMessageLen))
	daemon.logger.Info("", nil, "Stage: "+template, a...)
	daemon.lastStepTimestamp = time.Now().Unix()
}

// logPrintStage reports the start/finish of a maintenance step to the output buffer and program log.
func (daemon *Daemon) logPrintStageStep(out *bytes.Buffer, template string, a ...interface{}) {
	if duration := time.Now().Unix() - daemon.lastStepTimestamp; duration > 5 {
		out.WriteString(fmt.Sprintf("(it took %d seconds)\n", duration))
	}
	out.WriteString(lalog.TruncateString(fmt.Sprintf("---"+template+"\n", a...), lalog.MaxLogMessageLen))
	daemon.logger.Info("", nil, "Step: "+template, a...)
	daemon.lastStepTimestamp = time.Now().Unix()
}

// SystemMaintenance is a long routine that conducts comprehensive general system maintenance tasks.
func (daemon *Daemon) SystemMaintenance() string {
	out := new(bytes.Buffer)
	daemon.logPrintStage(out, "begin system maintenance")

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

	daemon.RunMaintenanceScripts(out)

	daemon.logPrintStage(out, "concluded system maintenance")
	return out.String()
}

// Run unit tests on the maintenance daemon. See TestMaintenance_Execute for daemon setup.
func TestMaintenance(check *Daemon, t testingstub.T) {
	defer os.RemoveAll(ReportFilePath)
	os.Remove(ReportFilePath)
	// Make sure maintenance is checking the ports and reporting their errors
	check.CheckTCPPorts = map[string][]int{"localhost": {11334}}
	if result, ok := check.Execute(context.Background()); ok || !strings.Contains(result, "Port errors") {
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
	if result, ok := check.Execute(context.Background()); !ok &&
		!strings.Contains(result, "Mail processor errors") &&
		!strings.Contains(result, "HTTP handler errors") {
		t.Fatal(result)
	}
	// Should have run pre script as well
	if _, err := os.Stat("/tmp/laitos-maintenance-pre-script-test"); err != nil {
		t.Fatal("did not run pre script")
	}
	// Break a feature
	check.ToolboxSelfTest.LookupByTrigger[".s"] = &toolbox.Shell{}
	if result, ok := check.Execute(context.Background()); ok || !strings.Contains(result, "Shell.SelfTest") { // broken shell configuration
		t.Fatal(result)
	}
	// Look for maintenance report in temporary file
	if content, err := os.ReadFile(ReportFilePath); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(string(content), "Shell.SelfTest") { // broken shell configuration
		t.Fatal(string(content))
	}
	check.ToolboxSelfTest.LookupByTrigger[".s"] = &toolbox.Shell{
		Unrestricted:    true,
		InterpreterPath: "/bin/bash",
	}
	// Expect checks to begin within a second
	if err := check.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Maintenance loop should successfully start within two seconds
	serverStopped := make(chan struct{}, 1)
	go func() {
		if err := check.StartAndBlock(); err != context.Canceled {
			t.Errorf("unexpected return value from daemon start: %+v", err)
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
