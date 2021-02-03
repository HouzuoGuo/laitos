package maintenance

import (
	"os"
	"strconv"

	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform/procexp"
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
	numUserModeSecInclChildren   *prometheus.GaugeVec
	numKernelModeSecInclChildren *prometheus.GaugeVec
	numRunSec                    *prometheus.GaugeVec
	numWaitSec                   *prometheus.GaugeVec
	numVoluntarySwitches         *prometheus.GaugeVec
	numInvoluntarySwitches       *prometheus.GaugeVec
}

// NewProcessExplorerMetrics creates a new ProcessExplorerMetrics with all of its metrics collectors initialised.
func NewProcessExplorerMetrics() *ProcessExplorerMetrics {
	if !misc.EnablePrometheusIntegration {
		return &ProcessExplorerMetrics{}
	}
	metricsLabelNames := []string{PrometheusProcessIDLabel}
	return &ProcessExplorerMetrics{
		numUserModeSecInclChildren:   prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_user_mode_sec_incl_children"}, metricsLabelNames),
		numKernelModeSecInclChildren: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_kernal_mode_sec_incl_children"}, metricsLabelNames),
		numRunSec:                    prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_run_sec"}, metricsLabelNames),
		numWaitSec:                   prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_wait_sec"}, metricsLabelNames),
		numVoluntarySwitches:         prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_voluntary_switches"}, metricsLabelNames),
		numInvoluntarySwitches:       prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "laitos_proc_num_involuntary_switches"}, metricsLabelNames),
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
	} {
		if err := prometheus.Register(metric); err != nil {
			return err
		}
	}
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
	return nil
}
