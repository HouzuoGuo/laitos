package tracing

import (
	"bytes"
	"io"
	"net"
	"testing"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FakeExternalProcessStarter struct {
	argv [][]string
}

func (fake *FakeExternalProcessStarter) StartProgram(env []string, timeout int, stdout io.WriteCloser, stderr io.WriteCloser, start chan<- error, terminate <-chan struct{}, program string, args ...string) error {
	close(start)
	_ = stdout.Close()
	_ = stderr.Close()
	fake.argv = append(fake.argv, append([]string{program}, args...))
	return nil
}

func TestActivityMonitor(t *testing.T) {
	fake := FakeExternalProcessStarter{}
	mon := NewActivityMonitor(lalog.DefaultLogger, 123, 456, fake.StartProgram)
	require.NoError(t, mon.InstallProbe(ProbeTcpProbe))
	want := []string{"bpftrace", "-f", "json", "-e", BpftraceCode[ProbeTcpProbe](123, 456)}
	require.Equal(t, [][]string{want}, fake.argv)
	// Repeatedly attempt to start the same probe.
	require.NoError(t, mon.InstallProbe(ProbeTcpProbe))
	require.Equal(t, [][]string{want}, fake.argv)
	// Stop and restart the probe.
	mon.StopProbe(ProbeTcpProbe)
	require.NoError(t, mon.InstallProbe(ProbeTcpProbe))
	require.Equal(t, [][]string{want, want}, fake.argv)
}

func TestProcProbe(t *testing.T) {
	fake := FakeExternalProcessStarter{}
	probe := NewProcProbe(lalog.DefaultLogger, 0, fake.StartProgram, "test code", 10)
	require.NoError(t, probe.Start())
	probe.Stop()
	require.Equal(t, [][]string{{"bpftrace", "-f", "json", "-e", "test code"}}, fake.argv)
}

func TestProbeSampleDeserialisation(t *testing.T) {
	probe := NewProcProbe(lalog.DefaultLogger, 0, nil, "not required", 10)
	in := bytes.NewReader([]byte(`
{"type": "map", "data": {"@tcp_src": {"[10,0,0,11,0,0,0,0,0,0,0,0,0,0,0,0,0,0,-1,-1,127,0,0,1,0,0,0,0],11": 12, "[2,0,-89,74,127,0,0,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],42826": 79}}}
{"type": "map", "data": {"@tcp_dest": {"[10,0,-89,66,0,0,0,0,0,0,0,0,0,0,0,0,0,0,-1,-1,127,0,0,1,0,0,0,0],42818": 123, "[2,0,0,11,127,0,0,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],11": 146}}}
`))
	probe.followStdout(in)
	assert.EqualValues(t, []BpfNetIOTrafficCounter{{IP: net.IPv4(127, 0, 0, 1), Port: 42826, ByteCounter: 79}, {IP: net.ParseIP("::ffff"), Port: 11, ByteCounter: 12}}, probe.Sample.TcpTrafficSources)
	assert.EqualValues(t, []BpfNetIOTrafficCounter{{IP: net.IPv4(127, 0, 0, 1), Port: 11, ByteCounter: 146}, {IP: net.ParseIP("::ffff"), Port: 42818, ByteCounter: 123}}, probe.Sample.TcpTrafficDestinations)
}
