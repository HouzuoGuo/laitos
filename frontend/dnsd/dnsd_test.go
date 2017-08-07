package dnsd

import (
	"encoding/hex"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestExtractDomainName(t *testing.T) {
	if name := ExtractDomainName(nil); !reflect.DeepEqual(name, []string{}) {
		t.Fatal(name)
	}
	if name := ExtractDomainName([]byte{}); !reflect.DeepEqual(name, []string{}) {
		t.Fatal(name)
	}
	if name := ExtractDomainName(githubComUDPQuery); !reflect.DeepEqual(name, []string{"github.com", "com"}) {
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
	match, err := hex.DecodeString("e575818000010001000000010667697468756203636f6d00000100010000291000000000000000c00c00010001000005ba000400000000")
	if err != nil {
		t.Fatal(err)
	}
	if packet := RespondWith0(githubComUDPQuery); !reflect.DeepEqual(packet, match) {
		t.Fatal(hex.EncodeToString(packet))
	}
}

func TestDNSD_DownloadBlacklists(t *testing.T) {
	daemon := DNSD{}
	if entries, err := daemon.GetAdBlacklistPGL(); err != nil || len(entries) < 100 {
		t.Fatal(err, entries)
	}
	if entries, err := daemon.GetAdBlacklistMVPS(); err != nil || len(entries) < 100 {
		t.Fatal(err, entries)
	}
}

func TestDNSD_StartAndBlockUDP(t *testing.T) {
	daemon := DNSD{}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen address") == -1 {
		t.Fatal(err)
	}
	daemon.Address = "127.0.0.1"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen port") == -1 {
		t.Fatal(err)
	}
	daemon.UDPPort = 62151
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "UDPForwarder") == -1 {
		t.Fatal(err)
	}
	daemon.UDPForwarder = "8.8.8.8:53"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "PerIPLimit") == -1 {
		t.Fatal(err)
	}
	daemon.PerIPLimit = 10
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "allowable IP") == -1 {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = []string{"192.", ""}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "any allowable IP") == -1 {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = []string{"192."}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// If run on Travis, my own IP won't be put into allowed query prefixes.
	if os.Getenv("TRAVIS") == "" && len(daemon.AllowQueryIPPrefixes) != 3 {
		// There should be three prefixes: 127., 192., and my IP
		t.Fatal("did not put my own IP into prefixes")
	}
	TestUDPQueries(&daemon, t)
}

func TestDNSD_StartAndBlockTCP(t *testing.T) {
	daemon := DNSD{}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen address") == -1 {
		t.Fatal(err)
	}
	daemon.Address = "127.0.0.1"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen port") == -1 {
		t.Fatal(err)
	}
	daemon.TCPPort = 18519
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "UDPForwarder") == -1 {
		t.Fatal(err)
	}
	daemon.TCPForwarder = "8.8.8.8:53"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "PerIPLimit") == -1 {
		t.Fatal(err)
	}
	daemon.PerIPLimit = 10
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "allowable IP") == -1 {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = []string{"192.", ""}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "any allowable IP") == -1 {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = []string{"192."}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// If run on Travis, my own IP won't be put into allowed query prefixes.
	if os.Getenv("TRAVIS") == "" && len(daemon.AllowQueryIPPrefixes) != 3 {
		// There should be three prefixes: 127., 192., and my IP
		t.Fatal("did not put my own IP into prefixes", daemon.AllowQueryIPPrefixes)
	}
	TestTCPQueries(&daemon, t)
}
