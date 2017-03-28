package dnsd

import (
	"bytes"
	"encoding/hex"
	"io/ioutil"
	"net"
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
	match, err := hex.DecodeString("e575818000010001000000010667697468756203636f6d00000100010000291000000000000000c00c00010001000005ba000400000000")
	if err != nil {
		t.Fatal(err)
	}
	if packet := RespondWith0(githubComUDPQuery); !reflect.DeepEqual(packet, match) {
		t.Fatal(hex.EncodeToString(packet))
	}
}

func TestDNSD_StartAndBlockUDP(t *testing.T) {
	daemon := DNSD{}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen address") == -1 {
		t.Fatal(err)
	}
	daemon.UDPListenAddress = "127.0.0.1"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen port") == -1 {
		t.Fatal(err)
	}
	daemon.UDPListenPort = 16321
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "ForwardTo") == -1 {
		t.Fatal(err)
	}
	daemon.UDPForwardTo = "8.8.8.8:53"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "PerIPLimit") == -1 {
		t.Fatal(err)
	}
	daemon.PerIPLimit = 10
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "allowable IP") == -1 {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = []string{"127", ""}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "any allowable IP") == -1 {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = []string{"127"}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(daemon.AllowQueryIPPrefixes) != 2 {
		t.Fatal("did not put my own IP into prefixes")
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
	packetBuf := make([]byte, MaxPacketSize)
	clientConn, err := net.DialUDP("udp", nil, serverAddr)
	// Try to reach rate limit
	var success int
	for i := 0; i < 20; i++ {
		if _, err := clientConn.Write(githubComUDPQuery); err != nil {
			t.Fatal(err)
		}
		clientConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		length, err := clientConn.Read(packetBuf)
		if err == nil && length > 50 {
			success++
		}
	}
	if success < 5 || success > 15 {
		t.Fatal(success)
	}
	// Wait out rate limit
	time.Sleep(RateLimitIntervalSec * time.Second)
	// Blacklist github and see if query gets a black hole response
	daemon.BlackList["github.com"] = struct{}{}
	if _, err := clientConn.Write(githubComUDPQuery); err != nil {
		t.Fatal(err)
	}
	clientConn.SetReadDeadline(time.Now().Add(2000 * time.Millisecond))
	respLen, err := clientConn.Read(packetBuf)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Index(packetBuf[:respLen], BlackHoleAnswer) == -1 {
		t.Fatal("did not answer black hole")
	}
}

func TestDNSD_StartAndBlockTCP(t *testing.T) {
	daemon := DNSD{}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen address") == -1 {
		t.Fatal(err)
	}
	daemon.TCPListenAddress = "127.0.0.1"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen port") == -1 {
		t.Fatal(err)
	}
	daemon.TCPListenPort = 16321
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "ForwardTo") == -1 {
		t.Fatal(err)
	}
	daemon.TCPForwardTo = "8.8.8.8:53"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "PerIPLimit") == -1 {
		t.Fatal(err)
	}
	daemon.PerIPLimit = 10
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "allowable IP") == -1 {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = []string{"127", ""}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "any allowable IP") == -1 {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = []string{"127"}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(daemon.AllowQueryIPPrefixes) != 2 {
		t.Fatal("did not put my own IP into prefixes")
	}
	// Server should start within two seconds
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)

	packetBuf := make([]byte, MaxPacketSize)
	var success int
	// Try to reach rate limit
	for i := 0; i < 20; i++ {
		clientConn, err := net.Dial("tcp", "127.0.0.1:16321")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := clientConn.Write(githubComTCPQuery); err != nil {
			t.Fatal(err)
		}
		clientConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		resp, err := ioutil.ReadAll(clientConn)
		if err == nil && len(resp) > 50 {
			success++
		}
		clientConn.Close()
	}
	if success < 5 || success > 15 {
		t.Fatal(success)
	}
	// Wait out rate limit
	time.Sleep(RateLimitIntervalSec * time.Second)
	// Blacklist github and see if query gets a black hole response
	daemon.BlackList["github.com"] = struct{}{}
	clientConn, err := net.Dial("tcp", "127.0.0.1:16321")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clientConn.Write(githubComTCPQuery); err != nil {
		t.Fatal(err)
	}
	clientConn.SetReadDeadline(time.Now().Add(2000 * time.Millisecond))
	respLen, err := clientConn.Read(packetBuf)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Index(packetBuf[:respLen], BlackHoleAnswer) == -1 {
		t.Fatal("did not answer black hole")
	}
}
