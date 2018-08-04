package plainsocket

import (
	"bufio"
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/HouzuoGuo/laitos/toolbox/filter"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

var TCPDurationStats = misc.NewStats() // TCPDurationStats stores statistics of duration of all TCP conversations.

/*
You may call this function only after having called Initialise()!
Start TCP daemon and block until daemon is told to stop.
*/
func (daemon *Daemon) StartAndBlockTCP() (err error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(daemon.Address, strconv.Itoa(daemon.TCPPort)))
	if err != nil {
		return fmt.Errorf("plainsocket.StartAndBlock: failed to listen on %s:%d - %v", daemon.Address, daemon.TCPPort, err)
	}
	defer listener.Close()
	daemon.tcpListener = listener
	// Process incoming TCP conversations
	daemon.logger.Info("StartAndBlockTCP", "", nil, "going to listen for connections")
	for {
		if misc.EmergencyLockDown {
			return misc.ErrEmergencyLockDown
		}
		clientConn, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			return fmt.Errorf("plainsocket.StartAndBlockTCP: failed to accept new connection - %v", err)
		}
		go daemon.HandleTCPConnection(clientConn)
	}
}

// Read a feature command from each input line, then invoke the requested feature and write the execution result back to client.
func (daemon *Daemon) HandleTCPConnection(clientConn net.Conn) {
	// Put processing duration (including IO time) into statistics
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		TCPDurationStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()
	defer clientConn.Close()
	clientIP := clientConn.RemoteAddr().(*net.TCPAddr).IP.String()
	// Check connection against rate limit even before reading a line of command
	if !daemon.rateLimit.Add(clientIP, true) {
		return
	}
	daemon.logger.Info("HandleTCPConnection", clientIP, nil, "working on the connection")
	// Allow up to 4MB of commands to be recieved per connection
	reader := textproto.NewReader(bufio.NewReader(io.LimitReader(clientConn, 4*1048576)))
	for {
		// Read one line of command that may be at most 1MB long
		clientConn.SetReadDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		line, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				daemon.logger.Warning("HandleTCPConnection", clientIP, err, "failed to read from client")
			}
			return
		}
		// Check against conversation rate limit
		if !daemon.rateLimit.Add(clientIP, true) {
			return
		}
		// Trim and ignore empty line
		line = textproto.TrimString(line)
		if line == "" {
			continue
		}
		// Process line of command and respond
		result := daemon.Processor.Process(toolbox.Command{Content: string(line), TimeoutSec: CommandTimeoutSec}, true)
		clientConn.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		clientConn.Write([]byte(result.CombinedOutput))
		clientConn.Write([]byte("\r\n"))
	}
}

// Run unit tests on the TCP server. See TestPlainTextProt_StartAndBlockUDP for daemon setup.
func TestTCPServer(server *Daemon, t testingstub.T) {
	if misc.HostIsWindows() {
		// FIXME: fix this test case for Windows
		t.Skip("FIXME: enable this test case for Windows")
	}
	// Prevent daemon from listening to UDP connections in this TCP test case
	udpListenPort := server.UDPPort
	server.UDPPort = 0
	defer func() {
		server.UDPPort = udpListenPort
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
		clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(server.TCPPort))
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
	if success < 1 || success > server.PerIPLimit*2 {
		t.Fatal("succeeded", success)
	}
	// Wait out rate limit (leave 3 seconds buffer for pending requests to complete)
	time.Sleep((RateLimitIntervalSec + 3) * time.Second)

	// Make two normal conversations
	clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(server.TCPPort))
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
	if string(badPINResp) != filter.ErrPINAndShortcutNotFound.Error() {
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
