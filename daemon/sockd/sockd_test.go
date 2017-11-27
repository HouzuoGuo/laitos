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
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen address") == -1 {
		t.Fatal(err)
	}
	daemon.Address = "127.0.0.1"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen port") == -1 {
		t.Fatal(err)
	}
	daemon.TCPPort = 8720
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "password") == -1 {
		t.Fatal(err)
	}
	daemon.Password = "abcdefg"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "PerIPLimit") == -1 {
		t.Fatal(err)
	}
	daemon.PerIPLimit = 10
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}

	TestSockd(&daemon, t)
}
