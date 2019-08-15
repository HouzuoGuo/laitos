package serialport

import (
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/daemon/common"
)

func TestSerialPortDaemon(t *testing.T) {
	daemon := Daemon{Processor: common.GetInsaneCommandProcessor()}
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
