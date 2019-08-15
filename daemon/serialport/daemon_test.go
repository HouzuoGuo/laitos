package serialport

import (
	"github.com/HouzuoGuo/laitos/misc"
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/daemon/common"
)

func TestSerialPortDaemon(t *testing.T) {
	if misc.HostIsWindows() {
		t.Log("The daemon is not compatible with windows, hence skipping the tests.")
		return
	}
	daemon := Daemon{DeviceGlobPatterns: []string{"&*(#@"}}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "malformed") == -1 {
		t.Fatal(err)
	}
	daemon.DeviceGlobPatterns = nil
	daemon.Processor = common.GetInsaneCommandProcessor()
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal(err)
	}
	// Good processor but empty patterns is acceptable
	daemon.Processor = common.GetTestCommandProcessor()
	if err := daemon.Initialise(); err != nil || daemon.PerDeviceLimit != 3 {
		t.Fatal(err)
	}
	TestDaemon(&daemon, t)
}
