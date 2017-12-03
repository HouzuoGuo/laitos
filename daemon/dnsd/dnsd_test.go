package dnsd

import (
	"encoding/hex"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestExtractDomainName(t *testing.T) {
	if name := ExtractDomainName(nil); name != "" {
		t.Fatal(name)
	}
	if name := ExtractDomainName([]byte{}); name != "" {
		t.Fatal(name)
	}
	if name := ExtractDomainName(githubComUDPQuery); name != "github.com" {
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
	if packet := RespondWith0(githubComUDPQuery); !reflect.DeepEqual(packet, match) {
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
	daemon.UpdateBlackList()
}

func TestDNSD(t *testing.T) {
	daemon := Daemon{}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "allowable IP") == -1 {
		t.Fatal(err)
	}
	// Test missing mandatory settings
	daemon.AllowQueryIPPrefixes = []string{"192.", ""}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "any allowable IP") == -1 {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = []string{"192."}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(daemon.AllowQueryIPPrefixes) != 4 {
		// There should be three prefixes: 127., ::1, 192., and my IP
		t.Fatal("did not put my own IP into prefixes")
	}
	// Test default settings
	if daemon.TCPPort != 53 || daemon.UDPPort != 53 || daemon.PerIPLimit != 100 || daemon.Address != "0.0.0.0" || !reflect.DeepEqual(daemon.Forwarders, DefaultForwarders) {
		t.Fatalf("%+v", daemon)
	}
	// Prepare settings for test
	daemon.Address = "127.0.0.1"
	daemon.UDPPort = 62151
	daemon.TCPPort = 18519
	daemon.PerIPLimit = 5
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}

	TestUDPQueries(&daemon, t)
	time.Sleep(RateLimitIntervalSec * time.Second)
	TestTCPQueries(&daemon, t)
}
