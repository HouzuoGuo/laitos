package dnsd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
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
	if entries, err := daemon.GetAdBlacklistPGL(); err != nil || len(entries) < 100 {
		t.Fatal(err, entries)
	}
	if entries, err := daemon.GetAdBlacklistMVPS(); err != nil || len(entries) < 100 {
		t.Fatal(err, entries)
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
	// Try to reach rate limit
	var success int
	for i := 0; i < 40; i++ {
		go func() {
			clientConn, err := net.DialUDP("udp", nil, serverAddr)
			if err != nil {
				t.Fatal(err)
			}
			defer clientConn.Close()
			if err := clientConn.SetDeadline(time.Now().Add((RateLimitIntervalSec - 1) * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := clientConn.Write(githubComUDPQuery); err != nil {
				t.Fatal(err)
			}
			length, err := clientConn.Read(packetBuf)
			fmt.Println("Read result", length, err)
			if err == nil && length > 50 {
				success++
			}
		}()
	}
	// Wait out rate limit
	time.Sleep(RateLimitIntervalSec * time.Second)
	if success < 5 || success > 15 {
		t.Fatal(success)
	}
	// Blacklist github and see if query gets a black hole response
	daemon.BlackList["github.com"] = struct{}{}
	// This test is flaky and I do not understand why
	var blackListSuccess bool
	for i := 0; i < 5; i++ {
		clientConn, err := net.DialUDP("udp", nil, serverAddr)
		if err != nil {
			t.Fatal(err)
		}
		if err := clientConn.SetDeadline(time.Now().Add((RateLimitIntervalSec - 1) * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := clientConn.Write(githubComUDPQuery); err != nil {
			t.Fatal(err)
		}
		respLen, err := clientConn.Read(packetBuf)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Index(packetBuf[:respLen], BlackHoleAnswer) != -1 {
			blackListSuccess = true
			break
		}
	}
	if !blackListSuccess {
		t.Fatal("did not answer to blacklist domain")
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
	for i := 0; i < 40; i++ {
		go func() {
			clientConn, err := net.Dial("tcp", "127.0.0.1:16321")
			if err != nil {
				t.Fatal(err)
			}
			defer clientConn.Close()
			if err := clientConn.SetDeadline(time.Now().Add((RateLimitIntervalSec - 1) * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := clientConn.Write(githubComTCPQuery); err != nil {
				t.Fatal(err)
			}
			resp, err := ioutil.ReadAll(clientConn)
			fmt.Println("Read result", len(resp), err)
			if err == nil && len(resp) > 50 {
				success++
			}
		}()
	}
	// Wait out rate limit
	time.Sleep(RateLimitIntervalSec * time.Second)
	if success < 5 || success > 15 {
		t.Fatal(success)
	}
	// Blacklist github and see if query gets a black hole response
	daemon.BlackList["github.com"] = struct{}{}
	// This test is flaky and I do not understand why
	var blackListSuccess bool
	for i := 0; i < 5; i++ {
		clientConn, err := net.Dial("tcp", "127.0.0.1:16321")
		if err != nil {
			t.Fatal(err)
		}
		if err := clientConn.SetDeadline(time.Now().Add((RateLimitIntervalSec - 1) * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := clientConn.Write(githubComTCPQuery); err != nil {
			t.Fatal(err)
		}
		respLen, err := clientConn.Read(packetBuf)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Index(packetBuf[:respLen], BlackHoleAnswer) != -1 {
			blackListSuccess = true
			break
		}
	}
	if !blackListSuccess {
		t.Fatal("did not answer to blacklist domain")
	}
}
