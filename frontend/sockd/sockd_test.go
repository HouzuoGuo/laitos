package sockd

import (
	"net"
	"strings"
	"testing"
	"time"
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
	var stopped bool
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stopped = true
	}()
	time.Sleep(2 * time.Second)
	if conn, err := net.Dial("tcp", "127.0.0.1:8720"); err != nil {
		t.Fatal(err)
	} else if n, err := conn.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}); err != nil && n != 10 {
		t.Fatal(err, n)
	}
	// Daemon should stop within a second
	daemon.Stop()
	time.Sleep(1 * time.Second)
	if !stopped {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	daemon.Stop()
	daemon.Stop()
}
