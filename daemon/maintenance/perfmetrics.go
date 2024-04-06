package maintenance

import (
	"os"
	"path"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
	"github.com/HouzuoGuo/laitos/platform/procexp"
	"github.com/HouzuoGuo/laitos/platform/tracing"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// PrometheusMetricExeLabel is the name of the metrics label indicating the executable name,
	// given to the metrics registered by laitos.
	PrometheusMetricExeLabel = "exe"
)

type ActivityMonitorMetrics struct {
	tcpConnectionCount *prometheus.GaugeVec
	tcpTrafficBytes    *prometheus.GaugeVec
	fdReadCount        *prometheus.GaugeVec
	fdReadBytes        *prometheus.GaugeVec
	fdWrittenCount     *prometheus.GaugeVec
	fdWrittenBytes     *prometheus.GaugeVec
	blockIOSectors     *prometheus.GaugeVec
	blockIOMillis      *prometheus.GaugeVec
}

type ActivityMonitorCollector struct {
	pid     int
	monitor *tracing.ActivityMonitor
	labels  prometheus.Labels
	probes  map[tracing.Probe]struct{}
	metrics *ActivityMonitorMetrics
	logger  *lalog.Logger
}

func (metrics *ActivityMonitorCollector) Start() {
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
			probeErr := metrics.monitor.InstallProbe(probe)
			metrics.logger.Info(metrics.pid, probeErr, "attempted to probe %v", tracepoints)
			if probeErr == nil {
				metrics.probes[probe] = struct{}{}
			}
		}
	}
}

func sumValues[K comparable, V int | float64](in map[K]V) V {
	var sum V
	for _, val := range in {
		sum += val
	}
	return sum
}

// Refresh reads the process activity monitor sample data collected over the scrape interval,
// and updates prometheus gauge values accordingly.
func (collector *ActivityMonitorCollector) Refresh() {
	for installedProbe := range collector.probes {
		probe := collector.monitor.RunningProbes[installedProbe]
		if probe == nil {
			continue
		}
		switch installedProbe {
		case tracing.ProbeSyscallRead:
			collector.metrics.fdReadCount.With(collector.labels).Set(float64(len(probe.Sample.FDBytesRead)))
			collector.metrics.fdReadBytes.With(collector.labels).Set(float64(sumValues(probe.Sample.FDBytesRead)))
		case tracing.ProbeSyscallWrite:
			collector.metrics.fdWrittenCount.With(collector.labels).Set(float64(len(probe.Sample.FDBytesWritten)))
			collector.metrics.fdWrittenBytes.With(collector.labels).Set(float64(sumValues(probe.Sample.FDBytesWritten)))
		case tracing.ProbeTcpProbe:
			collector.metrics.tcpConnectionCount.With(collector.labels).Set(float64(len(probe.Sample.TcpTrafficSources)))
			var sum int
			for _, entry := range probe.Sample.TcpTrafficSources {
				sum += entry.ByteCounter
			}
			collector.metrics.tcpTrafficBytes.With(collector.labels).Set(float64(sum))
		case tracing.ProbeBlockIO:
			collector.metrics.blockIOSectors.With(collector.labels).Set(float64(sumValues(probe.Sample.BlockDeviceIOSectors)))
			collector.metrics.blockIOMillis.With(collector.labels).Set(float64(sumValues(probe.Sample.BlockDeviceIONanos) / 1000000))
		}
	}
}

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

	activityMonitorMetrics *ActivityMonitorMetrics
	selfActivityMetrics    *ActivityMonitorCollector
	osActivityMetrics      *ActivityMonitorCollector

	logger     *lalog.Logger
	selfLabels prometheus.Labels
}

// NewProcessExplorerMetrics creates a new ProcessExplorerMetrics with all of its metrics collectors initialised.
func NewProcessExplorerMetrics(logger *lalog.Logger, scrapeIntervalSec int) *ProcessExplorerMetrics {
	if !misc.EnablePrometheusIntegration {
		return &ProcessExplorerMetrics{}
	}
	labels := []string{PrometheusMetricExeLabel}
	activityMetrics := &ActivityMonitorMetrics{
		tcpConnectionCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_tcp_connection_count"}, labels),
		tcpTrafficBytes:    prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_tcp_traffic_bytes"}, labels),
		fdReadCount:        prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_fd_read_count"}, labels),
		fdReadBytes:        prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_fd_read_bytes"}, labels),
		fdWrittenCount:     prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_fd_written_count"}, labels),
		fdWrittenBytes:     prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_fd_written_bytes"}, labels),
		blockIOSectors:     prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_block_io_sectors"}, labels),
		blockIOMillis:      prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_block_io_millis"}, labels),
	}
	exe, _ := os.Executable()
	var procID int
	proc, err := procexp.GetProcAndTaskStatus(0)
	if err == nil {
		procID = proc.Status.ProcessID
	}
	selfLabelValues := prometheus.Labels{PrometheusMetricExeLabel: path.Base(exe)}
	return &ProcessExplorerMetrics{
		numUserModeSecInclChildren:   prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_user_mode_sec_incl_children"}, labels),
		numKernelModeSecInclChildren: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_kernel_mode_sec_incl_children"}, labels),
		numRunSec:                    prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_run_sec"}, labels),
		numWaitSec:                   prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_wait_sec"}, labels),
		numVoluntarySwitches:         prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_voluntary_switches"}, labels),
		numInvoluntarySwitches:       prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_involuntary_switches"}, labels),

		activityMonitorMetrics: activityMetrics,
		selfActivityMetrics: &ActivityMonitorCollector{
			pid:     procID,
			monitor: tracing.NewActivityMonitor(logger, procID, scrapeIntervalSec, platform.StartProgram),
			labels:  selfLabelValues,
			probes:  make(map[tracing.Probe]struct{}),
			metrics: activityMetrics,
			logger:  logger,
		},
		osActivityMetrics: &ActivityMonitorCollector{
			pid:     procID,
			monitor: tracing.NewActivityMonitor(logger, 0, scrapeIntervalSec, platform.StartProgram),
			labels:  prometheus.Labels{PrometheusMetricExeLabel: ""},
			probes:  make(map[tracing.Probe]struct{}),
			metrics: activityMetrics,
			logger:  logger,
		},

		logger:     logger,
		selfLabels: selfLabelValues,
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

		metrics.activityMonitorMetrics.tcpConnectionCount,
		metrics.activityMonitorMetrics.tcpTrafficBytes,
		metrics.activityMonitorMetrics.fdReadCount,
		metrics.activityMonitorMetrics.fdReadBytes,
		metrics.activityMonitorMetrics.fdWrittenCount,
		metrics.activityMonitorMetrics.fdWrittenBytes,
		metrics.activityMonitorMetrics.blockIOSectors,
		metrics.activityMonitorMetrics.blockIOMillis,
	} {
		if err := prometheus.Register(metric); err != nil {
			return err
		}
	}
	metrics.selfActivityMetrics.Start()
	metrics.osActivityMetrics.Start()
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
	metrics.numUserModeSecInclChildren.With(metrics.selfLabels).Set(proc.Stats.NumUserModeSecInclChildren)
	metrics.numKernelModeSecInclChildren.With(metrics.selfLabels).Set(proc.Stats.NumKernelModeSecInclChildren)
	metrics.numRunSec.With(metrics.selfLabels).Set(proc.SchedulerStatsSum.NumRunSec)
	metrics.numWaitSec.With(metrics.selfLabels).Set(proc.SchedulerStatsSum.NumWaitSec)
	metrics.numVoluntarySwitches.With(metrics.selfLabels).Set(float64(proc.SchedulerStatsSum.NumVoluntarySwitches))
	metrics.numInvoluntarySwitches.With(metrics.selfLabels).Set(float64(proc.SchedulerStatsSum.NumInvoluntarySwitches))
	metrics.osActivityMetrics.Refresh()
	metrics.selfActivityMetrics.Refresh()
	return nil
}
