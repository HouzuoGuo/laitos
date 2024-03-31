package maintenance

import (
	"os"
	"strconv"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
	"github.com/HouzuoGuo/laitos/platform/procexp"
	"github.com/HouzuoGuo/laitos/platform/tracing"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// PrometheusProcessIDLabel is the name of data label given to process explorer metrics registered with prometheus.
	// The label data shall be the PID of this program.
	PrometheusProcessIDLabel = "pid"
	// PrometheusMetricsRefreshInterval is the interval at which process runtime
	// and activity metrics shall be refreshed by calling Refresh().
	PrometheusMetricsRefreshInterval = 10 * time.Second
)

// ProcessExplorerMetrics are the collection of program performance metrics registered with prometheus
// The measurements are taken from process status and statistics exposed by procfs (a Linux OS feature).
type ProcessExplorerMetrics struct {
	// The process runtime statistics are collected from /proc.
	numUserModeSecInclChildren   *prometheus.GaugeVec
	numKernelModeSecInclChildren *prometheus.GaugeVec
	numRunSec                    *prometheus.GaugeVec
	numWaitSec                   *prometheus.GaugeVec
	numVoluntarySwitches         *prometheus.GaugeVec
	numInvoluntarySwitches       *prometheus.GaugeVec

	// The activity statistics are collected with help from bpftrace.
	tcpSource              *prometheus.GaugeVec
	tcpDestination         *prometheus.GaugeVec
	fileDescriptorsRead    *prometheus.GaugeVec
	fileBytesRead          *prometheus.GaugeVec
	fileDescriptorsWritten *prometheus.GaugeVec
	fileBytesWritten       *prometheus.GaugeVec
	blockIOSectors         *prometheus.GaugeVec
	blockIOMillis          *prometheus.GaugeVec

	procMon *tracing.ActivityMonitor
}

// NewProcessExplorerMetrics creates a new ProcessExplorerMetrics with all of its metrics collectors initialised.
func NewProcessExplorerMetrics() *ProcessExplorerMetrics {
	if !misc.EnablePrometheusIntegration {
		return &ProcessExplorerMetrics{}
	}
	var selfProcessID int
	proc, err := procexp.GetProcAndTaskStatus(0)
	if err == nil {
		selfProcessID = proc.Status.ProcessID
	}
	metricsLabelNames := []string{PrometheusProcessIDLabel}
	return &ProcessExplorerMetrics{
		numUserModeSecInclChildren:   prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_user_mode_sec_incl_children"}, metricsLabelNames),
		numKernelModeSecInclChildren: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_kernel_mode_sec_incl_children"}, metricsLabelNames),
		numRunSec:                    prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_run_sec"}, metricsLabelNames),
		numWaitSec:                   prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_wait_sec"}, metricsLabelNames),
		numVoluntarySwitches:         prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_voluntary_switches"}, metricsLabelNames),
		numInvoluntarySwitches:       prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_involuntary_switches"}, metricsLabelNames),

		tcpSource:              prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_num_tcp_sources"}, metricsLabelNames),
		tcpDestination:         prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_num_tcp_destinations"}, metricsLabelNames),
		fileDescriptorsRead:    prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_num_fd_read"}, metricsLabelNames),
		fileBytesRead:          prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_file_bytes_read"}, metricsLabelNames),
		fileDescriptorsWritten: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_num_fd_written"}, metricsLabelNames),
		fileBytesWritten:       prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_file_bytes_written"}, metricsLabelNames),
		blockIOSectors:         prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_block_io_sectors"}, metricsLabelNames),
		blockIOMillis:          prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_block_io_millis"}, metricsLabelNames),

		procMon: tracing.NewActivityMonitor(lalog.DefaultLogger, selfProcessID, int(PrometheusMetricsRefreshInterval/time.Second), platform.StartProgram),
	}
}

// RegisterGlobally registers all program performance metrics with the global & default prometheus instance.
func (metrics *ProcessExplorerMetrics) RegisterGlobally() error {
	if !misc.EnablePrometheusIntegration {
		return nil
	}
	for _, metric := range []prometheus.Collector{
		metrics.numKernelModeSecInclChildren,
		metrics.numUserModeSecInclChildren,
		metrics.numRunSec,
		metrics.numWaitSec,
		metrics.numInvoluntarySwitches,
		metrics.numVoluntarySwitches,

		metrics.tcpSource,
		metrics.tcpDestination,
		metrics.fileDescriptorsRead,
		metrics.fileBytesRead,
		metrics.fileDescriptorsWritten,
		metrics.fileBytesWritten,
		metrics.blockIOSectors,
		metrics.blockIOMillis,
	} {
		if err := prometheus.Register(metric); err != nil {
			return err
		}
	}
	// TODO: start process activity probes.
	return nil
}

// Refresh reads the latest program performance measurements and gives them to prometheus metrics.
func (metrics *ProcessExplorerMetrics) Refresh() error {
	if !misc.EnablePrometheusIntegration {
		return nil
	}
	proc, err := procexp.GetProcAndTaskStatus(0)
	if err != nil {
		return err
	}
	labels := prometheus.Labels{PrometheusProcessIDLabel: strconv.Itoa(os.Getpid())}
	metrics.numUserModeSecInclChildren.With(labels).Set(proc.Stats.NumUserModeSecInclChildren)
	metrics.numKernelModeSecInclChildren.With(labels).Set(proc.Stats.NumKernelModeSecInclChildren)
	metrics.numRunSec.With(labels).Set(proc.SchedulerStatsSum.NumRunSec)
	metrics.numWaitSec.With(labels).Set(proc.SchedulerStatsSum.NumWaitSec)
	metrics.numVoluntarySwitches.With(labels).Set(float64(proc.SchedulerStatsSum.NumVoluntarySwitches))
	metrics.numInvoluntarySwitches.With(labels).Set(float64(proc.SchedulerStatsSum.NumInvoluntarySwitches))
	// TODO: set process activity gauge values.
	return nil
}
