package plain

import (
	"bufio"
	"fmt"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/global"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

/*
You may call this function only after having called Initialise()!
Start TCP daemon and block until daemon is told to stop.
*/
func (server *PlainTextDaemon) StartAndBlockTCP() (err error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", server.TCPListenAddress, server.TCPListenPort))
	if err != nil {
		return fmt.Errorf("PlainTextDaemon.StartAndBlock: failed to listen on %s:%d - %v", server.TCPListenAddress, server.TCPListenPort, err)
	}
	defer listener.Close()
	server.TCPListener = listener
	// Process incoming TCP conversations
	server.Logger.Printf("StartAndBlockTCP", "", nil, "going to listen for connections")
	for {
		if global.EmergencyLockDown {
			return global.ErrEmergencyLockDown
		}
		clientConn, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			return fmt.Errorf("PlainTextDaemon.StartAndBlockTCP: failed to accept new connection - %v", err)
		}
		go server.HandleTCPConnection(clientConn)
	}
}

// Read a feature command from each input line, then invoke the requested feature and write the execution result back to client.
func (server *PlainTextDaemon) HandleTCPConnection(clientConn net.Conn) {
	defer clientConn.Close()
	clientIP := clientConn.RemoteAddr().String()[:strings.LastIndexByte(clientConn.RemoteAddr().String(), ':')]
	// Check connection against rate limit even before reading a line of command
	if !server.RateLimit.Add(clientIP, true) {
		return
	}
	server.Logger.Printf("HandleTCPConnection", clientIP, nil, "working on the connection")
	reader := bufio.NewReader(clientConn)
	for {
		// Read one line of command
		clientConn.SetReadDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		line, _, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				server.Logger.Warningf("HandleTCPConnection", clientIP, err, "failed to read from client")
			}
			return
		}
		// Check against conversation rate limit
		if !server.RateLimit.Add(clientIP, true) {
			return
		}
		// Process line of command and respond
		result := server.Processor.Process(feature.Command{Content: string(line), TimeoutSec: CommandTimeoutSec})
		clientConn.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		clientConn.Write([]byte(result.CombinedOutput))
		clientConn.Write([]byte("\r\n"))
	}
}

// Run unit tests on the TCP server. See TestPlainTextProt_StartAndBlockUDP for daemon setup.
func TestTCPServer(server *PlainTextDaemon, t *testing.T) {
	// Prevent daemon from listening to UDP connections in this TCP test case
	udpListenPort := server.UDPListenPort
	server.UDPListenPort = 0
	defer func() {
		server.UDPListenPort = udpListenPort
	}()
	// Server should start within two seconds
	var stoppedNormally bool
	go func() {
		if err := server.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stoppedNormally = true
	}()
	time.Sleep(2 * time.Second)

	// Try to exceed rate limit
	success := 0
	for i := 0; i < 100; i++ {
		clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(server.TCPListenPort))
		if err != nil {
			t.Fatal(err)
		}
		_, err = clientConn.Write([]byte("verysecret .s echo hi\r\n"))
		if err != nil {
			t.Fatal(err)
		}
		goodPINResp, _, err := bufio.NewReader(clientConn).ReadLine()
		if err != nil {
			continue
		}
		if string(goodPINResp) != "hi" {
			t.Fatal(string(goodPINResp))
		}
		clientConn.Close()
		success++
	}
	if success < 5 || success > 15 {
		t.Fatal("succeeded", success)
	}
	// Wait till rate limit expires
	time.Sleep(RateLimitIntervalSec * time.Second)

	// Make two normal conversations
	clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(server.TCPListenPort))
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()
	reader := bufio.NewReader(clientConn)
	// Command with bad PIN
	_, err = clientConn.Write([]byte("pin mismatch\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	badPINResp, _, err := reader.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if string(badPINResp) != "Failed to match PIN/shortcut" {
		t.Fatal(string(badPINResp))
	}
	// With good PIN
	_, err = clientConn.Write([]byte("verysecret .s echo hi\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	goodPINResp, _, err := reader.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if string(goodPINResp) != "hi" {
		t.Fatal(string(goodPINResp))
	}
	// Daemon should stop within a second
	server.Stop()
	time.Sleep(1 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	server.Stop()
	server.Stop()
}
