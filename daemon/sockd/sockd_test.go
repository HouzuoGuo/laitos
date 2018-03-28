package sockd

import (
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"strings"
	"testing"
)

func TestSockd_StartAndBlock(t *testing.T) {
	daemon := Daemon{}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "dns daemon") == -1 {
		t.Fatal(err)
	}
	daemon.DNSDaemon = &dnsd.Daemon{}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen port") == -1 {
		t.Fatal(err)
	}
	daemon.TCPPort = 27101
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "password") == -1 {
		t.Fatal(err)
	}
	daemon.Password = "abcdefg"
	if err := daemon.Initialise(); err != nil || daemon.Address != "0.0.0.0" || daemon.PerIPLimit != 288 {
		t.Fatal(err)
	}

	daemon.Address = "127.0.0.1"
	daemon.TCPPort = 27101
	daemon.UDPPort = 13781
	daemon.Password = "abcdefg"
	daemon.PerIPLimit = 10

	TestSockd(&daemon, t)
}
