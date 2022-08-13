package dnsd

import (
	"context"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/toolbox"
)

func TestUpdateBlackList(t *testing.T) {
	daemon := Daemon{}
	daemon.Address = "127.0.0.1"
	daemon.UDPPort = 33111
	daemon.PerIPLimit = 5
	daemon.AllowQueryFromCidrs = []string{"192.168.0.0/8"}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// The parallel DNS resolution routines cannot handle a blacklist too small
	// with less than 12 entries.
	daemon.UpdateBlackList([]string{
		"apple.com", "github.com", "google.com", "microsoft.com",
		"apple.com", "github.com", "google.com", "microsoft.com",
		"apple.com", "github.com", "google.com", "microsoft.com",
		"apple.com", "github.com", "google.com", "microsoft.com",
	})
	// The blacklist contains both the domain names and the resolved IP addresses
	if len(daemon.blackList) < 4*2 {
		t.Fatal(len(daemon.blackList))
	}
}

func TestCheckAllowClientIP(t *testing.T) {
	daemon := Daemon{AllowQueryFromCidrs: []string{"192.0.0.0/8", "100.0.0.0/8"}}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Allowed by tags (client IPs) seen by store&forward message processor
	for _, client := range []string{"123.0.0.1", "123.0.0.2", "123.0.0.3"} {
		daemon.Processor.Features.MessageProcessor.StoreReport(context.Background(), toolbox.SubjectReportRequest{SubjectHostName: "dummy"}, client, "dummy")
		if !daemon.isRecursiveQueryAllowed(client) {
			t.Fatal("should have allowed", client)
		}
	}

	// Allowed by fast track
	for _, client := range []string{"127.0.0.1", "::1", "127.0.100.1", inet.GetPublicIP()} {
		if !daemon.isRecursiveQueryAllowed(client) {
			t.Fatal("should have allowed", client)
		}
	}
	// Allowed by configured prefixes
	for _, client := range []string{"192.168.1.1", "100.1.1.1"} {
		if !daemon.isRecursiveQueryAllowed(client) {
			t.Fatal("should have allowed", client)
		}
	}

	// Blocked
	for _, client := range []string{"172.16.0.1", "193.0.0.1", "101.0.0.1", "128.0.0.1", "1.1.1.2", "0.0.0.0", "123.0.0.5"} {
		if daemon.isRecursiveQueryAllowed(client) {
			t.Fatal("should have blocked", client)
		}
	}
}

func TestEmtpyDNSD(t *testing.T) {
	// DNS daemon must initialise without an error without any configuration
	daemon := &Daemon{}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// DNS blacklist must not crash even if it's empty and DNS daemon is not yet started.
	// Some other daemons (sockd, web proxy) borrow DNS daemon for its blacklist filtering.
	if daemon.IsInBlacklist("github.com") {
		t.Fatal("should not have been in the blacklist")
	}
}

func TestDaemon_Initialise(t *testing.T) {
	daemon := Daemon{AllowQueryFromCidrs: []string{"192.0.0.0/8", ""}, MyDomainNames: []string{"example.com"}}
	// Initialise with bad CIDR.
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "failed to parse") {
		t.Fatal(err)
	}
	// Fix CIDR.
	daemon.AllowQueryFromCidrs = nil
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(daemon.allowQueryFromCidrNets) != 0 {
		t.Fatal(daemon.allowQueryFromCidrNets)
	}
	// Initialise with a misconfigured command processor.
	daemon.Processor = toolbox.GetInsaneCommandProcessor()
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), toolbox.ErrBadProcessorConfig) {
		t.Fatal(err)
	}
	// Fix command processor.
	daemon.Processor = toolbox.GetTestCommandProcessor()
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Test default settings
	if daemon.TCPPort != 53 || daemon.UDPPort != 53 || daemon.PerIPLimit != 48 || daemon.Address != "0.0.0.0" || !reflect.DeepEqual(daemon.Forwarders, DefaultForwarders) {
		t.Fatalf("%+v", daemon)
	}
}

func TestDNSD(t *testing.T) {
	daemon := Daemon{
		Address:       "127.0.0.1",
		UDPPort:       62151,
		TCPPort:       18519,
		PerIPLimit:    100, // must be sufficient for test case
		MyDomainNames: []string{"example.com"},
	}
	daemon.Processor = toolbox.GetTestCommandProcessor()
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Prepare settings for test
	// Non-functioning forwarders should not abort initialisation or fail the daemon operation
	daemon.Forwarders = append(daemon.Forwarders, "does-not-exist:53", "also-does-not-exist:12")
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Configure daemon to use default set of recursive resolvers
	daemon.Forwarders = nil
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}

	TestServer(&daemon, t)
}

func TestDefaultForwarders(t *testing.T) {
	// The timeout is applied to all resolution attempts
	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Duration(60*time.Second))
	defer cancel()
	for _, forwarderAddr := range DefaultForwarders {
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (conn net.Conn, e error) {
				var d net.Dialer
				return d.DialContext(ctx, network, forwarderAddr)
			},
		}
		for _, name := range []string{"apple.com", "github.com", "google.com", "microsoft.com", "wikipedia.org"} {
			for i := 0; i < 10; i++ {
				addrs, err := resolver.LookupIPAddr(timeoutCtx, name)
				if err != nil {
					t.Fatalf("failed to resolve %s: %v", name, err)
				}
				if len(addrs) < 1 {
					t.Fatalf("resolver %q did not resolve %s to an address", forwarderAddr, name)
				}
			}
		}
	}
}

func TestDaemon_queryLabels(t *testing.T) {
	daemon := Daemon{
		MyDomainNames: []string{"a.com", "b.net.", "b.a.com"},
	}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(daemon.MyDomainNames, []string{".b.a.com", ".a.com", ".b.net"}) {
		t.Fatal(daemon.MyDomainNames)
	}
	var tests = []struct {
		name                string
		wantLabels          []string
		wantNumDomainLabels int
		wantIsRecursive     bool
	}{
		{
			name:                "",
			wantLabels:          []string{},
			wantNumDomainLabels: 0,
			wantIsRecursive:     false,
		},
		{
			name:                "haha.b.a.com",
			wantLabels:          []string{"haha"},
			wantNumDomainLabels: 3,
			wantIsRecursive:     false,
		},
		{
			name:                "haha.example.com",
			wantLabels:          []string{"haha", "example", "com"},
			wantNumDomainLabels: 0,
			wantIsRecursive:     true,
		},
		{
			name:                "c.b.a.com",
			wantLabels:          []string{"c"},
			wantNumDomainLabels: 3,
			wantIsRecursive:     false,
		},
		{
			name:                "c.d.b.net.",
			wantLabels:          []string{"c", "d"},
			wantNumDomainLabels: 2,
			wantIsRecursive:     false,
		},
	}
	for _, test := range tests {
		gotLabels, gotNumDomainLabels, gotRecursive := daemon.queryLabels(test.name)
		if !reflect.DeepEqual(gotLabels, test.wantLabels) {
			t.Errorf("name: %q, got labels: %+#v, want: %+#v", test.name, gotLabels, test.wantLabels)
		}
		if gotNumDomainLabels != test.wantNumDomainLabels {
			t.Errorf("name: %q, got number of domain labels: %v, want: %v", test.name, gotNumDomainLabels, test.wantNumDomainLabels)
		}
		if gotRecursive != test.wantIsRecursive {
			t.Errorf("name: %q, got recursive: %v, want: %v", test.name, gotRecursive, test.wantIsRecursive)
		}
	}
}
