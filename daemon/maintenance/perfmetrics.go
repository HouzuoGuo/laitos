package maintenance

import (
	"os"
	"strconv"

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
	tcpConnectionCount *prometheus.GaugeVec
	tcpTrafficBytes    *prometheus.GaugeVec
	fdReadCount        *prometheus.GaugeVec
	fdReadBytes        *prometheus.GaugeVec
	fdWrittenCount     *prometheus.GaugeVec
	fdWrittenBytes     *prometheus.GaugeVec
	blockIOSectors     *prometheus.GaugeVec
	blockIOMillis      *prometheus.GaugeVec

	logger            *lalog.Logger
	scrapeIntervalSec int
	procMon           *tracing.ActivityMonitor
	installedProbes   map[tracing.Probe]struct{}
}

// NewProcessExplorerMetrics creates a new ProcessExplorerMetrics with all of its metrics collectors initialised.
func NewProcessExplorerMetrics(logger *lalog.Logger, scrapeIntervalSec int) *ProcessExplorerMetrics {
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

		tcpConnectionCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_tcp_connection_count"}, metricsLabelNames),
		tcpTrafficBytes:    prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_tcp_traffic_bytes"}, metricsLabelNames),
		fdReadCount:        prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_fd_read_count"}, metricsLabelNames),
		fdReadBytes:        prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_fd_read_bytes"}, metricsLabelNames),
		fdWrittenCount:     prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_fd_written_count"}, metricsLabelNames),
		fdWrittenBytes:     prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_fd_written_bytes"}, metricsLabelNames),
		blockIOSectors:     prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_block_io_sectors"}, metricsLabelNames),
		blockIOMillis:      prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_block_io_millis"}, metricsLabelNames),

		logger:            logger,
		scrapeIntervalSec: scrapeIntervalSec,
		procMon:           tracing.NewActivityMonitor(lalog.DefaultLogger, selfProcessID, scrapeIntervalSec, platform.StartProgram),
		installedProbes:   make(map[tracing.Probe]struct{}),
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

		metrics.tcpConnectionCount,
		metrics.tcpTrafficBytes,
		metrics.fdReadCount,
		metrics.fdReadBytes,
		metrics.fdWrittenCount,
		metrics.fdWrittenBytes,
		metrics.blockIOSectors,
		metrics.blockIOMillis,
	} {
		if err := prometheus.Register(metric); err != nil {
			return err
		}
	}
	availableTracePoints := tracing.ListTracePoints()
	for probe, tracepoints := range tracing.ProbeNames {
		available := true
		for _, point := range tracepoints {
			if !availableTracePoints[point] {
				available = false
				break
			}
		}
		if available {
			probeErr := metrics.procMon.InstallProbe(probe)
			metrics.logger.Info(nil, probeErr, "attempted to install probe %v", tracepoints)
			if probeErr == nil {
				metrics.installedProbes[probe] = struct{}{}
			}
		}
	}
	return nil
}

func sumValues[K comparable, V int | float64](in map[K]V) V {
	var sum V
	for _, val := range in {
		sum += val
	}
	return sum
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
	// Read process activity monitor sample data over the scrape interval.
	// The activity monitor clears the sample data at the same interval.
	for installedProbe := range metrics.installedProbes {
		probe := metrics.procMon.RunningProbes[installedProbe]
		if probe == nil {
			continue
		}
		switch installedProbe {
		case tracing.ProbeSyscallRead:
			metrics.fdReadCount.With(labels).Set(float64(len(probe.Sample.FDBytesRead)))
			metrics.fdReadBytes.With(labels).Set(float64(sumValues(probe.Sample.FDBytesRead)))
		case tracing.ProbeSyscallWrite:
			metrics.fdWrittenCount.With(labels).Set(float64(len(probe.Sample.FDBytesWritten)))
			metrics.fdWrittenBytes.With(labels).Set(float64(sumValues(probe.Sample.FDBytesWritten)))
		case tracing.ProbeTcpProbe:
			metrics.tcpConnectionCount.With(labels).Set(float64(len(probe.Sample.TcpTrafficSources)))
			var sum int
			for _, entry := range probe.Sample.TcpTrafficSources {
				sum += entry.ByteCounter
			}
			metrics.tcpTrafficBytes.With(labels).Set(float64(sum))
		case tracing.ProbeBlockIO:
			metrics.blockIOSectors.With(labels).Set(float64(sumValues(probe.Sample.BlockDeviceIOSectors)))
			metrics.blockIOMillis.With(labels).Set(float64(sumValues(probe.Sample.BlockDeviceIONanos) / 1000000))
		}
	}
	return nil
}
