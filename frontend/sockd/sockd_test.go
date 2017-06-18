package sockd

import (
	"strings"
	"testing"
)

func TestSockd_StartAndBlock(t *testing.T) {
	daemon := Sockd{}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen address") == -1 {
		t.Fatal(err)
	}
	daemon.ListenAddress = "127.0.0.1"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen port") == -1 {
		t.Fatal(err)
	}
	daemon.ListenPort = 8720
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
