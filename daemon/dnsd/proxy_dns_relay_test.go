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
	// Start the DNS proxy server - it handles TCP-over-DNS traffic and acts as
	// a recursive DNS-over-TCP resolver.
	proxyServer := &Daemon{
		Address:             "127.0.0.1",
		AllowQueryFromCidrs: []string{"127.0.0.0/8"},
		PerIPLimit:          999,
		MyDomainNames:       []string{"example.test"},
		UDPPort:             22123,
		TCPPort:             22848,
		TCPProxy: &Proxy{
			RequestOTPSecret: "testtest",
			Debug:            true,
		},
	}
	if err := proxyServer.Initialise(); err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := proxyServer.StartAndBlock(); err != nil {
			panic(err)
		}
	}()
	if !misc.ProbePort(30*time.Second, proxyServer.Address, proxyServer.TCPPort) {
		t.Fatal("the daemon did not start on time")
	}
	// Start the DNS relay server.
	relay := &DNSRelay{
		Config: tcpoverdns.InitiatorConfig{
			SetConfig:               true,
			Debug:                   true,
			MaxSegmentLenExclHeader: MaxUpstreamSegmentLength(proxyServer.MyDomainNames[0]),
			Timing: tcpoverdns.TimingConfig{
				ReadTimeout:               MaxProxyConnectionLifetime,
				WriteTimeout:              MaxProxyConnectionLifetime,
				RetransmissionInterval:    5 * time.Second,
				SlidingWindowWaitDuration: 3000 * time.Millisecond,
				// Unlike the HTTP proxy, the timing of DNS relay needs
				// to be a bit tighter to be sufficiently responsive.
				KeepAliveInterval: 1000 * time.Millisecond,
				AckDelay:          100 * time.Millisecond,
			},
		},
		Debug:            true,
		RequestOTPSecret: proxyServer.TCPProxy.RequestOTPSecret,
		// This is the address used to reach the TCP-over-DNS daemon.
		DNSResolver: fmt.Sprintf("%s:%d", proxyServer.Address, proxyServer.UDPPort),
		DNSHostName: proxyServer.MyDomainNames[0],
		// This is the DNS-over-TCP resolver on the other side of TCP-over-DNS tunnel.
		ForwardTo: fmt.Sprintf("%s:%d", proxyServer.Address, proxyServer.TCPPort),
	}
	relayDaemon := &Daemon{
		Address:             "127.0.0.1",
		AllowQueryFromCidrs: []string{"0.0.0.0/0"},
		UDPPort:             58422,
		TCPPort:             58667,
		DNSRelay:            relay,
	}
	if err := relayDaemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	serverStopped := make(chan struct{}, 1)
	go func() {
		if err := relayDaemon.StartAndBlock(); err != nil {
			t.Errorf("unexpected return value from daemon start: %+v", err)
		}
		serverStopped <- struct{}{}
	}()
	if !misc.ProbePort(30*time.Second, relayDaemon.Address, relayDaemon.TCPPort) {
		t.Fatal("the daemon did not start on time")
	}

	tcpResolver := &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (conn net.Conn, e error) {
			return net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", relayDaemon.TCPPort))
		},
	}
	udpResolver := &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (conn net.Conn, e error) {
			return net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", relayDaemon.UDPPort))
		},
	}

	for i := 0; i < 5; i++ {
		addrs, err := tcpResolver.LookupIPAddr(context.Background(), "github.com")
		if err != nil {
			t.Fatalf("failed to resolve via TCP: %v", err)
		}
		fmt.Println(addrs)
	}
	for i := 0; i < 5; i++ {
		addrs, err := udpResolver.LookupIPAddr(context.Background(), "google.com")
		if err != nil {
			t.Fatalf("failed to resolve via UDP: %v", err)
		}
		fmt.Println(addrs)
	}
}
