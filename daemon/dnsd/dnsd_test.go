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
	daemon.AllowQueryIPPrefixes = []string{"192."}
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
	daemon := Daemon{AllowQueryIPPrefixes: []string{"192.", "100."}}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Allowed by tags (client IPs) seen by store&forward message processor
	for _, client := range []string{"123.0.0.1", "123.0.0.2", "123.0.0.3"} {
		daemon.Processor.Features.MessageProcessor.StoreReport(context.Background(), toolbox.SubjectReportRequest{SubjectHostName: "dummy"}, client, "dummy")
		if !daemon.checkAllowClientIP(client) {
			t.Fatal("should have allowed", client)
		}
	}

	// Allowed by fast track
	for _, client := range []string{"127.0.0.1", "::1", "127.0.100.1", inet.GetPublicIP()} {
		if !daemon.checkAllowClientIP(client) {
			t.Fatal("should have allowed", client)
		}
	}
	// Allowed by configured prefixes
	for _, client := range []string{"192.168.1.1", "100.1.1.1"} {
		if !daemon.checkAllowClientIP(client) {
			t.Fatal("should have allowed", client)
		}
	}

	// Blocked
	for _, client := range []string{"172.16.0.1", "193.0.0.1", "101.0.0.1", "128.0.0.1", "1.1.1.2", "0.0.0.0", "123.0.0.5"} {
		if daemon.checkAllowClientIP(client) {
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

func TestDNSD(t *testing.T) {
	daemon := Daemon{AllowQueryIPPrefixes: []string{"192.", ""}}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "may not contain empty string") {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = nil
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(daemon.AllowQueryIPPrefixes) != 0 {
		t.Fatal(daemon.AllowQueryIPPrefixes)
	}
	// Must not initialise if command processor is not sane
	daemon.Processor = toolbox.GetInsaneCommandProcessor()
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), toolbox.ErrBadProcessorConfig) {
		t.Fatal("did not error due to insane CommandProcessor")
	}
	daemon.Processor = toolbox.GetTestCommandProcessor()
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Test default settings
	if daemon.TCPPort != 53 || daemon.UDPPort != 53 || daemon.PerIPLimit != 48 || daemon.Address != "0.0.0.0" || !reflect.DeepEqual(daemon.Forwarders, DefaultForwarders) {
		t.Fatalf("%+v", daemon)
	}
	// Prepare settings for test
	daemon.Address = "127.0.0.1"
	daemon.UDPPort = 62151
	daemon.TCPPort = 18519
	daemon.PerIPLimit = 100 // must be sufficient for test case
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
