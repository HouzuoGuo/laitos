/*
plainsocket implements a Telnet-comaptible network service to provide unencrypted, plain-text access to all toolbox features.
Due to the unencrypted nature of this communication, users are strongly advised to utilise this service only as a last resort.
The implementation supports UDP as carrier of conversation in addition to TCP.
*/
package plainsocket

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"time"

	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/HouzuoGuo/laitos/toolbox/filter"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

const (
	IOTimeoutSec         = 60               // If a conversation goes silent for this many seconds, the connection is terminated.
	CommandTimeoutSec    = IOTimeoutSec - 1 // Command execution times out after this manys econds
	RateLimitIntervalSec = 1                // Rate limit is calculated at 1 second interval
)

// Daemon implements a Telnet-compatible service to provide unencrypted, plain-text access to all toolbox features, via both TCP and UDP.
type Daemon struct {
	Address    string                    `json:"Address"`    // Network address for both TCP and UDP to listen to, e.g. 0.0.0.0 for all network interfaces.
	TCPPort    int                       `json:"TCPPort"`    // TCP port to listen on
	UDPPort    int                       `json:"UDPPort"`    // UDP port to listen on
	PerIPLimit int                       `json:"PerIPLimit"` // PerIPLimit is approximately how many concurrent users are expected to be using the server from same IP address
	Processor  *toolbox.CommandProcessor `json:"-"`          // Feature command processor

	tcpServer *common.TCPServer
	udpServer *common.UDPServer
}

// Initialise validates configuration and initialises internal states.
func (daemon *Daemon) Initialise() error {
	if daemon.Processor == nil || daemon.Processor.IsEmpty() {
		return fmt.Errorf("plainsocket.Initialise: command processor and its filters must be configured")
	}
	if errs := daemon.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("plainsocket.Initialise: %+v", errs)
	}
	if daemon.Address == "" {
		daemon.Address = "0.0.0.0"
	}
	if daemon.PerIPLimit < 1 {
		daemon.PerIPLimit = 3 // reasonable for personal use
	}
	if daemon.UDPPort < 1 && daemon.TCPPort < 1 {
		// No reasonable defaults for these two, sorry.
		return errors.New("plainsocket.Initialise: either or both TCP and UDP ports must be specified and be greater than 0")
	}
	daemon.tcpServer = common.NewTCPServer(daemon.Address, daemon.TCPPort, "plainsocket", daemon, daemon.PerIPLimit)
	daemon.udpServer = common.NewUDPServer(daemon.Address, daemon.UDPPort, "plainsocket", daemon, daemon.PerIPLimit)
	return nil
}

// GetTCPStatsCollector returns stats collector for the TCP server of this daemon.
func (daemon *Daemon) GetTCPStatsCollector() *misc.Stats {
	return misc.PlainSocketStatsTCP
}

// HandleConnection converses with a TCP client.
func (daemon *Daemon) HandleTCPConnection(logger lalog.Logger, ip string, conn *net.TCPConn) {
	daemon.Processor.SetLogger(logger)
	// Allow up to 4MB of commands to be received per connection
	reader := textproto.NewReader(bufio.NewReader(io.LimitReader(conn, 4*1048576)))
	for {
		if misc.EmergencyLockDown {
			logger.Warning("HandleTCPConnection", "", misc.ErrEmergencyLockDown, "")
			return
		}
		// Read one line of command that may be at most 1MB long
		if err := conn.SetReadDeadline(time.Now().Add(IOTimeoutSec * time.Second)); err != nil {
			return
		}
		line, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				logger.Warning("HandleTCPConnection", ip, err, "failed to read from client")
			}
			return
		}
		// Check against conversation rate limit
		if !daemon.tcpServer.AddAndCheckRateLimit(ip) {
			return
		}
		// Trim and ignore empty line
		line = textproto.TrimString(line)
		if line == "" {
			continue
		}
		// Process line of command and respond
		result := daemon.Processor.Process(toolbox.Command{Content: string(line), TimeoutSec: CommandTimeoutSec}, true)
		if err := conn.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second)); err != nil {
			return
		} else if _, err := conn.Write([]byte(result.CombinedOutput)); err != nil {
			return
		} else if _, err := conn.Write([]byte("\r\n")); err != nil {
			return
		}
	}
}

// GetUDPStatsCollector returns stats collector for the UDP server of this daemon.
func (daemon *Daemon) GetUDPStatsCollector() *misc.Stats {
	return misc.PlainSocketStatsUDP
}

// Read a feature command from each input line, then invoke the requested feature and write the execution result back to client.
func (daemon *Daemon) HandleUDPClient(logger lalog.Logger, ip string, client *net.UDPAddr, packet []byte, srv *net.UDPConn) {
	daemon.Processor.SetLogger(logger)
	reader := textproto.NewReader(bufio.NewReader(bytes.NewReader(packet)))
	for {
		if misc.EmergencyLockDown {
			logger.Warning("HandleUDPClient", "", misc.ErrEmergencyLockDown, "")
			return
		}
		// Read one line of command
		line, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				logger.Warning("HandleUDPClient", ip, err, "failed to read received packet")
			}
			return
		}
		// Check against conversation rate limit
		if !daemon.udpServer.AddAndCheckRateLimit(ip) {
			return
		}
		// Trim and ignore empty line
		line = textproto.TrimString(line)
		if line == "" {
			continue
		}
		// Process line of command and respond
		result := daemon.Processor.Process(toolbox.Command{Content: string(line), TimeoutSec: CommandTimeoutSec}, true)
		if err := srv.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second)); err != nil {
			logger.Warning("HandleUDPClient", ip, err, "failed to write response")
			return
		} else if _, err := srv.WriteToUDP([]byte(result.CombinedOutput+"\r\n"), client); err != nil {
			logger.Warning("HandleUDPClient", ip, err, "failed to write response")
			return
		}
	}
}

// StartAndBLock starts both TCP and UDP listeners. You may call this function only after having called Initialise().
func (daemon *Daemon) StartAndBlock() error {
	numListeners := 0
	errChan := make(chan error, 2)
	if daemon.TCPPort != 0 {
		numListeners++
		go func() {
			err := daemon.tcpServer.StartAndBlock()
			errChan <- err
		}()
	}
	if daemon.UDPPort != 0 {
		numListeners++
		go func() {
			err := daemon.udpServer.StartAndBlock()
			errChan <- err
		}()
	}
	for i := 0; i < numListeners; i++ {
		if err := <-errChan; err != nil {
			daemon.Stop()
			return err
		}
	}
	return nil
}

// Close all of open TCP and UDP listeners so that they will cease processing incoming connections.
func (daemon *Daemon) Stop() {
	daemon.tcpServer.Stop()
	daemon.udpServer.Stop()
}

// TestServer contains the comprehensive test case for both TCP and UDP servers.
func TestServer(server *Daemon, t testingstub.T) {
	// Server should start within two seconds
	var stoppedNormally bool
	go func() {
		if err := server.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stoppedNormally = true
	}()
	time.Sleep(2 * time.Second)

	// Prepare for TCP conversations
	tcpClient, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(server.TCPPort))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = tcpClient.Close()
	}()
	reader := bufio.NewReader(tcpClient)
	// Command with bad PIN
	_, err = tcpClient.Write([]byte("pin mismatch\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	badPINResp, _, err := reader.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if string(badPINResp) != toolbox.ErrPINAndShortcutNotFound.Error() {
		t.Fatal(string(badPINResp))
	}
	// With good PIN
	_, err = tcpClient.Write([]byte("verysecret .s echo hi\r\n"))
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

	// Prepare for UDP conversations
	udpClient, err := net.Dial("udp", "127.0.0.1:"+strconv.Itoa(server.UDPPort))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = udpClient.Close()
	}()
	reader = bufio.NewReader(udpClient)
	// Command with bad PIN
	_, err = udpClient.Write([]byte("pin mismatch\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	badPINResp, _, err = reader.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if string(badPINResp) != toolbox.ErrPINAndShortcutNotFound.Error() {
		t.Fatal(string(badPINResp))
	}
	// With good PIN
	_, err = udpClient.Write([]byte("verysecret .s echo hi\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	goodPINResp, _, err = reader.ReadLine()
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
