package tracing

import (
	"io"
	"strings"
	"sync"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/platform"
)

// Probe identifies a supported tracing probe.
type Probe int

const (
	ProbeSyscallRead = Probe(iota)
	ProbeSyscallWrite
	ProbeTcpProbe
	ProbeBlockIO
)

var (
	// ProbeName maps all supported probe types to their bpftrace probe names.
	ProbeName = map[Probe][]string{
		ProbeSyscallRead:  {"tracepoint:syscalls:sys_enter_read", "tracepoint:syscalls:sys_exit_read"},
		ProbeSyscallWrite: {"tracepoint:syscalls:sys_enter_write", "tracepoint:syscalls:sys_exit_write"},
		ProbeTcpProbe:     {"tracepoint:tcp:tcp_probe"},
		ProbeBlockIO:      {"tracepoint:block:block_io_start", "tracepoint:block:block_io_done"},
	}
)

// ListProbes returns the list of probes supported by bpftrace.
func ListProbes() []string {
	out, err := platform.InvokeProgram(nil, 10, "bpftrace", "-l")
	if err != nil {
		lalog.DefaultLogger.Warning(nil, err, "failed to execute bpftrace")
		return nil
	}
	return strings.Split(out, "\n")
}

type ProcProbe struct {
	// ScriptCode is the bpftrace script of this probe.
	ScriptCode string

	logger      *lalog.Logger
	termChan    chan struct{}
	procStarter platform.ExternalProcessStarter
}

// NewProcProbe returns a newly initialised process probe. Caller needs to call Start to start gathering data.
func NewProcProbe(logger *lalog.Logger, scriptCode string, procStarter platform.ExternalProcessStarter) (ret *ProcProbe) {
	ret = &ProcProbe{
		ScriptCode:  scriptCode,
		logger:      logger,
		procStarter: procStarter,
	}
	return
}

func (probe *ProcProbe) followStdout(reader io.Reader) {

}

func (probe *ProcProbe) followStderr(reader io.Reader) {

}

// Start the probe's bpftrace external program and follows the output for the latest samping statistics.
// Does not wait for program's completion.
func (probe *ProcProbe) Start() {
	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()
	probe.termChan = make(chan struct{})
	go probe.followStdout(outReader)
	go probe.followStderr(errReader)
	go func() {
		err := probe.procStarter(nil, 1<<30, outWriter, errWriter, make(chan struct{}), probe.termChan, "bpftrace", "-e", probe.ScriptCode, "-f", "json")
		probe.logger.Warning("ProcProbe", err, "bpftrace has exited")
	}()
}

// Stop the probe's bpftrace external program.
func (probe *ProcProbe) Stop() {
	close(probe.termChan)
}

// ActivityMonitor uses eBPF probes to sample system and process activities at regular intervals.
type ActivityMonitor struct {
	// PID is the ID of process being traced. All probes share the same PID.
	PID int
	// RunningProbes consist of the bpftrace probes running in separate processes.
	RunningProbes map[Probe]*ProcProbe
	// SamplingIntervalSec is the tracing data sampling rate in seconds. All probes share the same sampling interval.
	SamplingIntervalSec int

	logger      *lalog.Logger
	mutex       *sync.Mutex
	procStarter platform.ExternalProcessStarter
}

// NewActivityMonitor returns a new initialised process activity monitor.
func NewActivityMonitor(logger *lalog.Logger, pid int, samplingIntervalSec int, procStarter platform.ExternalProcessStarter) (ret *ActivityMonitor) {
	ret = &ActivityMonitor{
		PID:                 pid,
		RunningProbes:       make(map[Probe]*ProcProbe),
		SamplingIntervalSec: samplingIntervalSec,
		logger:              logger,
		procStarter:         procStarter,
		mutex:               new(sync.Mutex),
	}
	return
}

// StartProbe starts a new probe. It does nothing if the probe is already running.
func (mon *ActivityMonitor) InstallProbe(probe Probe) {
	mon.mutex.Lock()
	defer mon.mutex.Unlock()
	if _, exists := mon.RunningProbes[probe]; exists {
		return
	}
	p := NewProcProbe(mon.logger, "", mon.procStarter)
	mon.RunningProbes[probe] = p
	p.Start()
}

// StopProbe stops a probe. It does nothing if the probe is not running.
func (mon *ActivityMonitor) StopProbe(probe Probe) {
	mon.mutex.Lock()
	defer mon.mutex.Unlock()
	if p, exists := mon.RunningProbes[probe]; exists {
		p.Stop()
	}
	delete(mon.RunningProbes, probe)
}
