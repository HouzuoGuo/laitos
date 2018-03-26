package plainsocket

import (
	"github.com/HouzuoGuo/laitos/daemon/common"
	"strings"
	"testing"
	"time"
)

func TestPlainTextDaemon(t *testing.T) {
	daemon := Daemon{}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "filters must be configured") == -1 {
		t.Fatal(err)
	}
	daemon.Processor = common.GetInsaneCommandProcessor()
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal(err)
	}
	daemon.Processor = common.GetTestCommandProcessor()
	// Test missing mandatory settings
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "TCP and UDP ports") == -1 {
		t.Fatal(err)
	}
	// Test default settings
	daemon.TCPPort = 32789
	daemon.UDPPort = 15890
	if err := daemon.Initialise(); err != nil || daemon.PerIPLimit != 1 {
		t.Fatalf("%+v %+v\n", err, daemon)
	}
	// Prepare settings for test
	daemon.Address = "127.0.0.1"
	daemon.PerIPLimit = 5 // limit must be high enough to tolerate consecutive command tests
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	TestTCPServer(&daemon, t)
	time.Sleep(RateLimitIntervalSec * time.Second)
	TestUDPServer(&daemon, t)
}
