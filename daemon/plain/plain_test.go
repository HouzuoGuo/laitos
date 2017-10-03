package plain

import (
	"github.com/HouzuoGuo/laitos/daemon/common"
	"strings"
	"testing"
)

func TestPlainTextDaemon_StartAndBlockTCP(t *testing.T) {
	daemon := PlainTextDaemon{Processor: common.GetTestCommandProcessor()}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen address") == -1 {
		t.Fatal(err)
	}
	daemon.Address = "127.0.0.1"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen port") == -1 {
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
	daemon := PlainTextDaemon{Processor: common.GetTestCommandProcessor()}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen address") == -1 {
		t.Fatal(err)
	}
	daemon.Address = "127.0.0.1"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen port") == -1 {
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
