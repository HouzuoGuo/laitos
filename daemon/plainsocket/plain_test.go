package plainsocket

import (
	"github.com/HouzuoGuo/laitos/daemon/common"
	"strings"
	"testing"
)

func TestPlainTextDaemon_StartAndBlockTCP(t *testing.T) {
	daemon := Daemon{}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "filters must be configured") == -1 {
		t.Fatal(err)
	}
	daemon.Processor = common.GetInsaneCommandProcessor()
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal(err)
	}
	daemon.Processor = common.GetTestCommandProcessor()
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen address") == -1 {
		t.Fatal(err)
	}
	daemon.Address = "127.0.0.1"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "TCP and UDP ports") == -1 {
		t.Fatal(err)
	}
	daemon.TCPPort = 32789
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "PerIPLimit") == -1 {
		t.Fatal(err)
	}
	daemon.PerIPLimit = 10
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	TestTCPServer(&daemon, t)
}

func TestPlainTextDaemon_StartAndBlockUDP(t *testing.T) {
	daemon := Daemon{Processor: common.GetTestCommandProcessor()}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen address") == -1 {
		t.Fatal(err)
	}
	daemon.Address = "127.0.0.1"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "TCP and UDP ports") == -1 {
		t.Fatal(err)
	}
	daemon.UDPPort = 15890
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "PerIPLimit") == -1 {
		t.Fatal(err)
	}
	daemon.PerIPLimit = 10
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	TestUDPServer(&daemon, t)
}
