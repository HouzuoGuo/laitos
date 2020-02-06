package plainsocket

import (
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/toolbox"
)

func TestPlainTextDaemon(t *testing.T) {
	daemon := Daemon{}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "filters must be configured") {
		t.Fatal(err)
	}
	daemon.Processor = toolbox.GetInsaneCommandProcessor()
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), toolbox.ErrBadProcessorConfig) {
		t.Fatal(err)
	}
	daemon.Processor = toolbox.GetTestCommandProcessor()
	// Test missing mandatory settings
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "TCP and UDP ports") {
		t.Fatal(err)
	}
	// Test default settings
	daemon.TCPPort = 32789
	daemon.UDPPort = 15890
	if err := daemon.Initialise(); err != nil || daemon.PerIPLimit != 3 {
		t.Fatalf("%+v %+v\n", err, daemon)
	}
	// Prepare settings for test
	daemon.Address = "127.0.0.1"
	daemon.PerIPLimit = 5 // limit must be high enough to tolerate consecutive command tests
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	TestServer(&daemon, t)
}
