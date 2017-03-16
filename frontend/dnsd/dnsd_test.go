package dnsd

import (
	"encoding/hex"
	"net"
	"strings"
	"testing"
	"time"
)

func TestDNSD_StartAndBlock(t *testing.T) {
	daemon := DNSD{}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen address") == -1 {
		t.Fatal(err)
	}
	daemon.ListenAddress = "127.0.0.1"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen port") == -1 {
		t.Fatal(err)
	}
	daemon.ListenPort = 16321
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "ForwardTo") == -1 {
		t.Fatal(err)
	}
	daemon.ForwardTo = "8.8.8.8"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "PerIPLimit") == -1 {
		t.Fatal(err)
	}
	daemon.PerIPLimit = 10
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "allowable IP") == -1 {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = []string{"127", ""}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "all allowable IP prefixes") == -1 {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = []string{"127"}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Update ad-server blacklist
	if numEntries, err := daemon.InstallAdBlacklist(); err != nil || numEntries < 100 {
		t.Fatal(err, numEntries)
	}
	// Server should start within two seconds
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)

	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:16321")
	if err != nil {
		t.Fatal(err)
	}
	githubComQuery, err := hex.DecodeString("97eb010000010000000000000667697468756203636f6d0000010001")
	if err != nil {
		t.Fatal(err)
	}
	packetBuf := make([]byte, MaxPacketSize)
	clientConn, err := net.DialUDP("udp", nil, serverAddr)
	// Try to reach rate limit
	delete(daemon.BlackList, "github.com")
	var success int
	for i := 0; i < 100; i++ {
		if _, err := clientConn.Write(githubComQuery); err != nil {
			t.Fatal(err)
		}
		clientConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		length, err := clientConn.Read(packetBuf)
		if err == nil && length > 50 {
			success++
		}
	}
	if success < 8 || success > 12 {
		t.Fatal(success)
	}
	// Wait out rate limit
	time.Sleep(RateLimitIntervalSec * time.Second)
	// Blacklist github and see if query still succeeds
	daemon.BlackList["github.com"] = struct{}{}
	if _, err := clientConn.Write(githubComQuery); err != nil {
		t.Fatal(err)
	}
	clientConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if _, err = clientConn.Read(packetBuf); err == nil {
		t.Fatal("did not block")
	}
}
