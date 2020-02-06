package serialport

import (
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/toolbox"
)

func TestSerialPortDaemon(t *testing.T) {
	daemon := Daemon{Processor: toolbox.GetInsaneCommandProcessor()}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), toolbox.ErrBadProcessorConfig) {
		t.Fatal(err)
	}
	// Good processor but empty patterns is acceptable
	daemon.Processor = toolbox.GetTestCommandProcessor()
	if err := daemon.Initialise(); err != nil || daemon.PerDeviceLimit != 3 {
		t.Fatal(err)
	}
	TestDaemon(&daemon, t)
}
