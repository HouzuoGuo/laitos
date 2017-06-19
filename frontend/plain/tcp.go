package plain

import (
	"bufio"
	"fmt"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/global"
	"net"
	"strings"
	"testing"
	"time"
)

/*
You may call this function only after having called Initialise()!
Start TCP daemon and block until daemon is told to stop.
*/
func (server *PlainText) StartAndBlockTCP() (err error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", server.TCPListenAddress, server.TCPListenPort))
	if err != nil {
		return fmt.Errorf("PlainText.StartAndBlock: failed to listen on %s:%d - %v", server.TCPListenAddress, server.TCPListenPort, err)
	}
	defer listener.Close()
	server.Listener = listener
	// Process incoming TCP conversations
	server.Logger.Printf("StartAndBlock", "", nil, "going to listen for connections")
	for {
		if global.EmergencyLockDown {
			return global.ErrEmergencyLockDown
		}
		clientConn, err := server.Listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			return fmt.Errorf("PlainText.StartAndBlock: failed to accept new connection - %v", err)
		}
		go server.HandleTCPConnection(clientConn)
	}
}

// Read a feature command from each input line, then invoke the requested feature and write the execution result back to client.
func (server *PlainText) HandleTCPConnection(clientConn net.Conn) {
	defer clientConn.Close()
	clientIP := clientConn.RemoteAddr().String()[:strings.LastIndexByte(clientConn.RemoteAddr().String(), ':')]
	// Check connection against rate limit even before reading a line of command
	if !server.RateLimit.Add(clientIP, true) {
		return
	}
	reader := bufio.NewReader(clientConn)
	for {
		// Read one line of command
		clientConn.SetReadDeadline(time.Now().Add(IOTimeoutSec))
		line, _, err := reader.ReadLine()
		if err != nil {
			server.Logger.Warningf("HandleTCPConnection", clientIP, err, "failed to read from client")
			return
		}
		// Check against conversation rate limit
		if !server.RateLimit.Add(clientIP, true) {
			return
		}
		// Process line of command and respond
		result := server.Processor.Process(feature.Command{Content: string(line), TimeoutSec: server.CommandTimeoutSec})
		clientConn.SetWriteDeadline(time.Now().Add(IOTimeoutSec))
		clientConn.Write([]byte(result.CombinedOutput))
		clientConn.Write([]byte("\r\n"))
	}
}

// Run unit tests on the TCP server. See TestPlainText_StartAndBlockTCP for daemon setup.
func TestPlainTCPServer(server *PlainText, t *testing.T) {
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
