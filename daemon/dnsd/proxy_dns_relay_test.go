package dnsd

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
)

func TestDNSRelay(t *testing.T) {
	t.Skip("TODO FIXME: fix this test")
	// Start the DNS proxy server.
	dnsProxyServer := &Daemon{
		Address:             "127.0.0.1",
		AllowQueryFromCidrs: []string{"127.0.0.0/8"},
		PerIPLimit:          999,
		MyDomainNames:       []string{"example.test"},
		UDPPort:             45278,
		TCPPort:             32148,
		TCPProxy: &Proxy{
			RequestOTPSecret: "testtest",
		},
	}
	if err := dnsProxyServer.Initialise(); err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := dnsProxyServer.StartAndBlock(); err != nil {
			panic(err)
		}
	}()
	if !misc.ProbePort(30*time.Second, dnsProxyServer.Address, dnsProxyServer.TCPPort) {
		t.Fatal("DNS proxy server did not start on time")
	}
	// Start the DNS relay server.
	relay := &DNSRelay{
		Config: tcpoverdns.InitiatorConfig{
			SetConfig:               true,
			MaxSegmentLenExclHeader: 111,
			Timing: tcpoverdns.TimingConfig{
				ReadTimeout:       20 * time.Second,
				WriteTimeout:      20 * time.Second,
				KeepAliveInterval: 5 * time.Second,
			},
			Debug: true,
		},
		Debug:            true,
		RequestOTPSecret: dnsProxyServer.TCPProxy.RequestOTPSecret,
		DNSResolver:      fmt.Sprintf("%s:%d", dnsProxyServer.Address, dnsProxyServer.UDPPort),
		DNSHostName:      dnsProxyServer.MyDomainNames[0],
		ForwardTo:        "8.8.8.8:53",
	}
	daemon := &Daemon{
		Address:             "127.0.0.1",
		AllowQueryFromCidrs: []string{"0.0.0.0/0"},
		UDPPort:             58422,
		TCPPort:             19211,
		DNSRelay:            relay,
	}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	serverStopped := make(chan struct{}, 1)
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Errorf("unexpected return value from daemon start: %+v", err)
		}
		serverStopped <- struct{}{}
	}()
	if !misc.ProbePort(30*time.Second, daemon.Address, daemon.TCPPort) {
		t.Fatal("did not start within two seconds")
	}

	tcpResolver := &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (conn net.Conn, e error) {
			return net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", daemon.TCPPort))
		},
	}
	udpResolver := &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (conn net.Conn, e error) {
			return net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", daemon.UDPPort))
		},
	}
	addrs, err := tcpResolver.LookupAddr(context.Background(), "github.com")
	if err != nil {
		t.Fatalf("failed to resolve via TCP: %v", err)
	}
	fmt.Println(addrs)
	addrs, err = udpResolver.LookupAddr(context.Background(), "github.com")
	if err != nil {
		t.Fatalf("failed to resolve via UDP: %v", err)
	}
	fmt.Println(addrs)
}
