package plain

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/global"
	"github.com/HouzuoGuo/laitos/ratelimit"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Check configuration and initialise internal states.
func (server *PlainText) Initialise() error {
	server.Logger = global.Logger{ComponentName: "PlainText", ComponentID: fmt.Sprintf("%s:%d", server.ListenAddress, server.ListenPort)}
	if errs := server.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("HTTPD.Initialise: %+v", errs)
	}
	if server.ListenAddress == "" {
		return errors.New("PlainText.Initialise: listen address must not be empty")
	}
	if server.ListenPort < 1 {
		return errors.New("PlainText.Initialise: listen port must be greater than 0")
	}
	server.RateLimit = &ratelimit.RateLimit{
		MaxCount: server.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
		Logger:   server.Logger,
	}
	server.RateLimit.Initialise()
	return nil
}

/*
You may call this function only after having called Initialise()!
Start TCP daemon and block until daemon is told to stop.
*/
func (server *PlainText) StartAndBlock() (err error) {
	server.Logger.Printf("StartAndBlock", "", nil, "going to listen for connections")
	server.Listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", server.ListenAddress, server.ListenPort))
	if err != nil {
		return fmt.Errorf("PlainText.StartAndBlock: failed to listen on %s:%d - %v", server.ListenAddress, server.ListenPort, err)
	}
	for {
		if global.EmergencyLockDown {
			return global.ErrEmergencyLockDown
		}
		clientConn, err := server.Listener.Accept()
		if err != nil {
			// Listener is told to stop
			if strings.Contains(err.Error(), "closed") {
				return nil
			} else {
				return fmt.Errorf("PlainText.StartAndBlock: failed to accept new connection - %v", err)
			}
		}
		go server.HandleConnection(clientConn)
	}
}

/*
Read one feature command from every line of input. After a line is read, the feature is invoked and
execution result is written back to the client.
*/
func (server *PlainText) HandleConnection(clientConn net.Conn) {
	clientIP := clientConn.RemoteAddr().String()[:strings.LastIndexByte(clientConn.RemoteAddr().String(), ':')]
	if !server.RateLimit.Add(clientIP, true) {
		return
	}
	defer clientConn.Close()
	reader := bufio.NewReader(clientConn)
	for i := 0; i < MaxConversations; i++ {
		clientConn.SetReadDeadline(time.Now().Add(IOTimeoutSec))
		line, _, err := reader.ReadLine()
		if err != nil {
			server.Logger.Warningf("HandleConnection", clientIP, err, "failed to read from client")
			return
		}
		if !server.RateLimit.Add(clientIP, true) {
			return
		}
		result := server.Processor.Process(feature.Command{Content: string(line), TimeoutSec: server.CommandTimeoutSec})
		clientConn.SetWriteDeadline(time.Now().Add(IOTimeoutSec))
		clientConn.Write([]byte(result.CombinedOutput))
	}
}

// If server daemon has started (i.e. listener is set), close the listener so that its connection loop will terminate.
func (server *PlainText) Stop() {
	if server.Listener != nil {
		if err := server.Listener.Close(); err != nil {
			server.Logger.Warningf("Stop", "", err, "failed to close listener")
		}
	}
}

// Run unit tests on the health checker. See TestHealthCheck_Execute for daemon setup.
func TestPlainTCPServer(server *PlainText, t *testing.T) {
	var stopped bool
	// Expect daemon to start within two seconds
	go func() {
		if err := server.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stopped = true
	}()
	time.Sleep(2 * time.Second)

	if conn, err := net.Dial("tcp", server.ListenAddress+":"+strconv.Itoa(server.ListenPort)); err != nil {
		t.Fatal(err)
	}

	// Daemon should stop within a second
	server.Stop()
	time.Sleep(1 * time.Second)
	if !stopped {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	server.Stop()
	server.Stop()
}
