package dnsd

import (
	"encoding/hex"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
)

func TestExtractDomainName(t *testing.T) {
	if name := ExtractDomainName(nil); name != "" {
		t.Fatal(name)
	}
	if name := ExtractDomainName([]byte{}); name != "" {
		t.Fatal(name)
	}
	if name := ExtractDomainName(GithubComUDPQuery); name != "github.coM" {
		t.Fatal(name)
	}
}

func TestRespondWith0(t *testing.T) {
	if packet := RespondWith0(nil); len(packet) != 0 {
		t.Fatal(packet)
	}
	if packet := RespondWith0([]byte{}); len(packet) != 0 {
		t.Fatal(packet)
	}
	match, err := hex.DecodeString("e575818000010001000000010667697468756203636f4d00000100010000291000000000000000c00c00010001000005ba000400000000")
	if err != nil {
		t.Fatal(err)
	}
	if packet := RespondWith0(GithubComUDPQuery); !reflect.DeepEqual(packet, match) {
		t.Fatal(hex.EncodeToString(packet))
	}
}

func TestUpdateBlackList(t *testing.T) {
	daemon := Daemon{}
	daemon.Address = "127.0.0.1"
	daemon.UDPPort = 33111
	daemon.PerIPLimit = 5
	daemon.AllowQueryIPPrefixes = []string{"192."}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	daemon.UpdateBlackList(2000)
	// Assuming that half of them successfully resolve into IP address
	if len(daemon.blackList) < 3000 {
		t.Fatal(len(daemon.blackList))
	}
}

func TestCheckAllowClientIP(t *testing.T) {
	daemon := Daemon{AllowQueryIPPrefixes: []string{"192.", "100."}}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{"127.0.0.1", "::1", "127.0.100.1", "192.168.0.1", "100.0.0.0", inet.GetPublicIP()} {
		if !daemon.checkAllowClientIP(client) {
			t.Fatal("should have allowed", client)
		}
	}
	for _, client := range []string{"172.16.0.1", "193.0.0.1", "101.0.0.1", "128.0.0.1", "1.1.1.2"} {
		if daemon.checkAllowClientIP(client) {
			t.Fatal("should have blocked", client)
		}
	}
}

func TestDNSD(t *testing.T) {
	daemon := Daemon{AllowQueryIPPrefixes: []string{"192.", ""}}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "may not contain empty string") == -1 {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = nil
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(daemon.AllowQueryIPPrefixes) != 0 {
		t.Fatal(daemon.AllowQueryIPPrefixes)
	}
	// Test default settings
	if daemon.TCPPort != 53 || daemon.UDPPort != 53 || daemon.PerIPLimit != 48 || daemon.Address != "0.0.0.0" || !reflect.DeepEqual(daemon.Forwarders, DefaultForwarders) {
		t.Fatalf("%+v", daemon)
	}
	// Prepare settings for test
	daemon.Address = "127.0.0.1"
	daemon.UDPPort = 62151
	daemon.TCPPort = 18519
	daemon.PerIPLimit = 5
	// Non-functioning forwarders should not abort initialisation or fail the daemon operation
	daemon.Forwarders = append(daemon.Forwarders, "does-not-exist:53", "also-does-not-exist:12")
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}

	TestUDPQueries(&daemon, t)
	time.Sleep(RateLimitIntervalSec * time.Second)
	TestTCPQueries(&daemon, t)
}
