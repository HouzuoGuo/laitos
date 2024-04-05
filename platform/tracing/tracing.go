package tracing

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/platform"
)

// Probe identifies a supported tracing probe.
// Only one instance of each probe may be attached globally or to an individual PID at a time.
type Probe int

const (
	ProbeSyscallRead = Probe(iota)
	ProbeSyscallWrite
	ProbeTcpProbe
	ProbeBlockIO
)

// ListTracePoints returns the list of probes supported by bpftrace.
func ListTracePoints() map[string]bool {
	ret := make(map[string]bool)
	out, err := platform.InvokeProgram(nil, 10, "bpftrace", "-l")
	if err != nil {
		lalog.DefaultLogger.Warning(nil, err, "failed to execute bpftrace")
		return nil
	}
	for _, probe := range strings.Split(out, "\n") {
		ret[probe] = true
	}
	return ret
}

type ProcProbe struct {
	// ScriptCode is the bpftrace script of this probe.
	ScriptCode string
	// SamplingIntervalSec is the tracing data sampling rate in seconds.
	SamplingIntervalSec int
	// Sample is the latest sample data deserialised from bpftrace output.
	Sample BpfSample

	latestSample time.Time
	mutex        *sync.Mutex
	logger       *lalog.Logger
	stopChan     chan struct{}
	procStarter  platform.ExternalProcessStarter
}

// NewProcProbe returns a newly initialised process probe. Caller needs to call Start to start gathering data.
func NewProcProbe(logger *lalog.Logger, procStarter platform.ExternalProcessStarter, scriptCode string, samplingIntervalSec int) (ret *ProcProbe) {
	ret = &ProcProbe{
		ScriptCode:          scriptCode,
		SamplingIntervalSec: samplingIntervalSec,
		logger:              logger,
		procStarter:         procStarter,
		mutex:               new(sync.Mutex),
		stopChan:            make(chan struct{}),
	}
	return
}

// followStdout continously deserialises the latest sample data from bpftrace's stdout.
func (probe *ProcProbe) followStdout(in io.Reader) {
	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		var rec BpfMapRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec.Type == "map" && rec.Data != nil {
			if read := rec.Data["@read_fd"]; read != nil {
				probe.mutex.Lock()
				probe.Sample.FDBytesRead = read
				probe.latestSample = time.Now()
				probe.mutex.Unlock()
			} else if written := rec.Data["@write_fd"]; written != nil {
				probe.mutex.Lock()
				probe.Sample.FDBytesWritten = written
				probe.latestSample = time.Now()
				probe.mutex.Unlock()
			} else if tcpSrc := rec.Data["@tcp_src"]; tcpSrc != nil {
				probe.mutex.Lock()
				probe.Sample.TcpTrafficSources = TcpTrafficFromBpfMap(tcpSrc)
				probe.latestSample = time.Now()
				probe.mutex.Unlock()
			} else if tcpDest := rec.Data["@tcp_dest"]; tcpDest != nil {
				probe.mutex.Lock()
				probe.Sample.TcpTrafficDestinations = TcpTrafficFromBpfMap(tcpDest)
				probe.latestSample = time.Now()
				probe.mutex.Unlock()
			} else if duration := rec.Data["@blkdev_dur"]; duration != nil {
				probe.mutex.Lock()
				probe.Sample.BlockDeviceIONanos = duration
				probe.latestSample = time.Now()
				probe.mutex.Unlock()
			} else if sectors := rec.Data["@blkdev_sector_count"]; sectors != nil {
				probe.mutex.Lock()
				probe.Sample.BlockDeviceIOSectors = sectors
				probe.latestSample = time.Now()
				probe.mutex.Unlock()
			}
		}
	}
}

func (probe *ProcProbe) followStderr(in io.Reader) {
	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		probe.logger.Info("", nil, "bpftrace stderr: %q", line)
	}
}

func (probe *ProcProbe) housekeeping() {
	interval := time.Duration(probe.SamplingIntervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// In the absence of new trace point activities, clear outdated sample data.
			probe.mutex.Lock()
			if time.Since(probe.latestSample) > interval {
				probe.Sample = BpfSample{}
			}
			probe.mutex.Unlock()
		case <-probe.stopChan:
			return
		}
	}
}

// Start executes bpftrace and returns to the caller. It does not wait for bpftrace to complete.
func (probe *ProcProbe) Start() error {
	probe.mutex.Lock()
	defer probe.mutex.Unlock()
	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()
	startChan := make(chan error)
	go func() {
		err := probe.procStarter(nil, 1<<30, outWriter, errWriter, startChan, probe.stopChan, "bpftrace", "-f", "json", "-e", probe.ScriptCode)
		probe.logger.Warning("ProcProbe", err, "bpftrace has exited")
	}()
	err := <-startChan
	if err != nil {
		return err
	}
	go probe.followStdout(outReader)
	go probe.followStderr(errReader)
	go probe.housekeeping()
	return nil
}

// Stop the probe's bpftrace process.
func (probe *ProcProbe) Stop() {
	probe.mutex.Lock()
	defer probe.mutex.Unlock()
	close(probe.stopChan)
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
func (mon *ActivityMonitor) InstallProbe(probe Probe) error {
	mon.mutex.Lock()
	defer mon.mutex.Unlock()
	if _, exists := mon.RunningProbes[probe]; exists {
		return nil
	}
	p := NewProcProbe(mon.logger, mon.procStarter, BpftraceCode[probe](mon.PID, mon.SamplingIntervalSec), mon.SamplingIntervalSec)
	mon.RunningProbes[probe] = p
	return p.Start()
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
