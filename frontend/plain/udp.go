package plain

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/global"
	"net"
	"strings"
	"testing"
	"time"
)

const (
	MaxPacketSize = 9038 // Maximum acceptable UDP packet size
)

/*
You may call this function only after having called Initialise()!
Start UDP daemon and block until daemon is told to stop.
*/
func (server *PlainText) StartAndBlockUDP() error {
	listenAddr := fmt.Sprintf("%s:%d", server.UDPListenAddress, server.UDPListenPort)
	udpAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return err
	}
	udpServer, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	defer udpServer.Close()
	server.UDPListener = udpServer
	server.Logger.Printf("StartAndBlockUDP", listenAddr, nil, "going to listen for queries")
	// Process incoming requests
	packetBuf := make([]byte, MaxPacketSize)
	for {
		if global.EmergencyLockDown {
			return global.ErrEmergencyLockDown
		}
		packetLength, clientAddr, err := udpServer.ReadFromUDP(packetBuf)
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			return fmt.Errorf("PlainText.StartAndBlockUDP: failed to accept new connection - %v", err)
		}
		// Check address against rate limit
		clientIP := clientAddr.IP.String()
		if !server.RateLimit.Add(clientIP, true) {
			continue
		}

		clientPacket := make([]byte, packetLength)
		copy(clientPacket, packetBuf[:packetLength])
		go server.HandleUDPConnection(clientIP, clientAddr, clientPacket)
	}
}

// Read a feature command from each input line, then invoke the requested feature and write the execution result back to client.
func (server *PlainText) HandleUDPConnection(clientIP string, clientAddr *net.UDPAddr, packet []byte) {
	listener := server.UDPListener
	if listener == nil {
		server.Logger.Warningf("HandleUDPConnection", clientIP, nil, "listener is closed before request can be processed")
		return
	}
	// Check connection against rate limit even before reading a line of command
	if !server.RateLimit.Add(clientIP, true) {
		return
	}
	reader := bufio.NewReader(bytes.NewReader(packet))
	for {
		// Read one line of command
		line, _, err := reader.ReadLine()
		if err != nil {
			server.Logger.Warningf("HandleUDPConnection", clientIP, err, "impossible has happened - byte reader failed")
			return
		}
		// Check against conversation rate limit
		if !server.RateLimit.Add(clientIP, true) {
			return
		}
		// Process line of command and respond
		result := server.Processor.Process(feature.Command{Content: string(line), TimeoutSec: server.CommandTimeoutSec})
		server.UDPListener.SetWriteDeadline(time.Now().Add(IOTimeoutSec))
		server.UDPListener.WriteToUDP([]byte(result.CombinedOutput), clientAddr)
		server.UDPListener.WriteToUDP([]byte("\r\n"), clientAddr)
	}
}

// Run unit tests on the UDP server. See TestPlainText_StartAndBlockUDP for daemon setup.
func TestPlainUDPServer(server *PlainText, t *testing.T) {
	// Prevent daemon from listening to TCP connections in this UDP test case
	tcpListenPort := server.TCPListenPort
	server.TCPListenPort = 0
	defer func() {
		server.TCPListenPort = tcpListenPort
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
