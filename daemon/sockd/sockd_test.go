package sockd

import (
	"net"
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/daemon/dnsd"
)

func TestSockd_StartAndBlock(t *testing.T) {
	daemon := Daemon{}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "dns daemon") {
		t.Fatal(err)
	}
	daemon.DNSDaemon = &dnsd.Daemon{}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "listen port") {
		t.Fatal(err)
	}
	daemon.TCPPorts = []int{27101}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatal(err)
	}
	daemon.Password = "abcdefg"
	if err := daemon.Initialise(); err != nil || daemon.Address != "0.0.0.0" || daemon.PerIPLimit != 96 {
		t.Fatal(err)
	}

	daemon.Address = "127.0.0.1"
	daemon.TCPPorts = []int{27101, 23990}
	daemon.UDPPorts = []int{13781, 38191}
	daemon.Password = "abcdefg"
	daemon.PerIPLimit = 10
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}

	TestSockd(&daemon, t)
}

func TestIsReservedAddr(t *testing.T) {
	notReserved := []net.IP{
		net.IPv4(8, 8, 8, 8),
		net.IPv4(193, 0, 0, 1),
		net.IPv4(1, 1, 1, 1),
		net.IPv4(54, 0, 0, 0),
	}
	for _, addr := range notReserved {
		if IsReservedAddr(addr) {
			t.Fatal(addr.String())
		}
	}

	reserved := []net.IP{
		net.IPv4(10, 0, 0, 1),
		net.IPv4(100, 64, 0, 1),
		net.IPv4(127, 0, 0, 1),
		net.IPv4(169, 254, 0, 1),
		net.IPv4(172, 16, 0, 1),
		net.IPv4(192, 0, 0, 1),
		net.IPv4(192, 0, 2, 1),
		net.IPv4(192, 168, 0, 1),
		net.IPv4(198, 18, 0, 1),
		net.IPv4(198, 51, 100, 1),
		net.IPv4(203, 0, 113, 1),
		net.IPv4(240, 0, 0, 1),
		net.IPv4(240, 0, 0, 95),
	}
	for _, addr := range reserved {
		if !IsReservedAddr(addr) {
			t.Fatal(addr.String())
		}
	}
}
