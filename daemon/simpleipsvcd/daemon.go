/*
simpleip implements simple & standard Internet services that were used in the nostalgic era of computing.
According to those protocol standard, the services are available via TCP and UDP simultaneously:
- sysstat (active system user name on port 11, RFC 866)
- daytime (current system time in readable text on port 12+1, RFC 867)
- QOTD (short text message as quote of the day on port 17, RFC 865)
*/
package simpleipsvcd

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
)

const (
	IOTimeoutSec         = 30 // IOTimeoutSec is the timeout used in network request/response operations.
	RateLimitIntervalSec = 1  // RateLimitIntervalSec is the interval for rate limit calculation.
)

// Daemon implements simple & standard Internet services that were used in the nostalgic era of computing.
type Daemon struct {
	Address         string `json:"Address"`        // Address to listen on, e.g. 0.0.0.0 to listen on all network interfaces.
	PerIPLimit      int    `json:"PerIPLimit"`     // PerIPLimit is approximately how many requests are allowed from an IP within a designated interval.
	ActiveUserNames string `json:"ActiveUserName"` // ActiveUserNames are CRLF-separated list of user names to appear in the response of "sysstat" network service.
	QOTD            string `json:"QOTD"`           // QOTD is the message to appear in the response of "QOTD" network service.

	rateLimit *misc.RateLimit
	logger    lalog.Logger

	// tcpServers, udpServers, and serverResponseFun contain listener structures and their corresponding response content function.
	tcpServers        map[int]net.Listener
	udpServers        map[int]*net.UDPConn
	serverResponseFun map[int]func() string
}

// Initialise validates configuration and initialises internal states.
func (daemon *Daemon) Initialise() error {
	if daemon.Address == "" {
		daemon.Address = "0.0.0.0"
	}
	if daemon.PerIPLimit < 1 {
		// The default is sufficient for 1 request per second on all three services via both TCP and UDP
		daemon.PerIPLimit = 6
	}
	daemon.ActiveUserNames = strings.TrimSpace(daemon.ActiveUserNames)
	daemon.QOTD = strings.TrimSpace(daemon.QOTD)

	daemon.logger = lalog.Logger{
		ComponentName: "simpleip",
		ComponentID:   []lalog.LoggerIDField{{"Addr", daemon.Address}},
	}
	daemon.rateLimit = &misc.RateLimit{
		MaxCount: daemon.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
		Logger:   daemon.logger,
	}
	daemon.rateLimit.Initialise()

	daemon.tcpServers = make(map[int]net.Listener)
	daemon.udpServers = make(map[int]*net.UDPConn)
	daemon.serverResponseFun = map[int]func() string{11: daemon.responseActiveUsers, 12 + 1: daemon.responseDayTime, 17: daemon.responseQOTD}
	return nil
}

// StartAndBlock starts all TCP and UDP servers to serve network clients. You may call this function only after having called Initialise().
func (daemon *Daemon) StartAndBlock() error {
	// There are 3 TCP servers and 3 UDP servers
	wg := new(sync.WaitGroup)
	wg.Add(6)
	// 11 - active users; 12+1 - daytime; 17 - QOTD
	for _, port := range []int{11, 12 + 1, 17} {
		// Start TCP listener on the port
		tcpServer, err := net.Listen("tcp", fmt.Sprintf("%s:%d", daemon.Address, port))
		if err != nil {
			daemon.Stop()
			return fmt.Errorf("simpleip.StartAndBlock: failed to start TCP server on port %d - %v", port, err)
		}
		daemon.tcpServers[port] = tcpServer
		go func(port int) {
			daemon.tcpResponderLoop(port)
			wg.Done()
		}(port)

		// Start UDP server on the port
		udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", daemon.Address, port))
		if err != nil {
			daemon.Stop()
			return fmt.Errorf("simpleip.StartAndBlock: failed to resolve UDP listen address on port %d - %v", port, err)
		}
		udpServer, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			daemon.Stop()
			return fmt.Errorf("simpleip.StartAndBlock: failed to start UDP server on port %d - %v", port, err)
		}
		daemon.udpServers[port] = udpServer
		go func(port int) {
			daemon.udpResponderLoop(port)
			wg.Done()
		}(port)
	}
	// Wait for servers to be stop
	wg.Wait()
	return nil
}

// Stop terminates all TCP and UDP servers.
func (daemon *Daemon) Stop() {
	if daemon.tcpServers != nil {
		for port, server := range daemon.tcpServers {
			if server == nil {
				continue
			}
			if err := server.Close(); err != nil {
				daemon.logger.Warning("Stop", strconv.Itoa(port), err, "failed to stop TCP server")
			}
		}
	}
	daemon.tcpServers = map[int]net.Listener{}
	if daemon.udpServers != nil {
		for port, server := range daemon.udpServers {
			if server == nil {
				continue
			}
			if err := server.Close(); err != nil {
				daemon.logger.Warning("Stop", strconv.Itoa(port), err, "failed to stop UDP server")
			}
		}
	}
	daemon.udpServers = map[int]*net.UDPConn{}
}

// responseActiveUsers returns configured active system user names in response to a sysstat service client.
func (daemon *Daemon) responseActiveUsers() string {
	return daemon.ActiveUserNames + "\r\n"
}

// responseDayTime returns the current system time inn RFC3339 format in response to a daytime service client.
func (daemon *Daemon) responseDayTime() string {
	return time.Now().Format(time.RFC3339) + "\r\n"
}

// responseQOTD returns the configured QOTD in response to a QOTD service client.
func (daemon *Daemon) responseQOTD() string {
	return daemon.QOTD + "\r\n"
}

// tcpResponderLoop blocks caller to server all incoming TCP connections on the service port, until its listener is stopped.
func (daemon *Daemon) tcpResponderLoop(port int) {
	logFunName := fmt.Sprintf("tcpResponderLoop-%d", port)
	for {
		if misc.EmergencyLockDown {
			daemon.logger.Warning(logFunName, "", nil, "quit due to EmergencyLockDown")
			return
		}
		clientConn, err := daemon.tcpServers[port].Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return
			}
			daemon.logger.Warning(logFunName, fmt.Sprintf("Port-%d", port), err, "quit due to error")
			return
		}
		// Check IP address against rate limit
		clientIP := clientConn.RemoteAddr().(*net.TCPAddr).IP.String()
		if !daemon.rateLimit.Add(clientIP, true) {
			clientConn.Close()
			continue
		}
		go func(clientConn net.Conn) {
			// Put processing duration (including IO time) into statistics
			beginTimeNano := time.Now().UnixNano()
			defer func() {
				common.SimpleIPStatsTCP.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
			}()
			defer clientConn.Close()
			daemon.logger.Info(logFunName, clientIP, nil, "working on the request")
			clientConn.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second))
			if _, err := clientConn.Write([]byte(daemon.serverResponseFun[port]())); err != nil {
				daemon.logger.Warning(logFunName, clientIP, err, "failed to write response")
			}
		}(clientConn)
	}
}

// udpResponderLoop blocks caller to serve all incoming UDP requests on the service port, until its server is stopped.
func (daemon *Daemon) udpResponderLoop(port int) {
	logFunName := fmt.Sprintf("udpResponderLoop-%d", port)
	udpServer := daemon.udpServers[port]
	for {
		if misc.EmergencyLockDown {
			daemon.logger.Warning(logFunName, "", nil, "quit due to EmergencyLockDown")
			return
		}
		// The simple IP services do not ask client to send anything meaningful
		discardBuf := make([]byte, 1500)
		_, clientAddr, err := udpServer.ReadFromUDP(discardBuf)
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return
			}
			daemon.logger.Warning(logFunName, "", err, "quit due to error")
			return
		}
		// Check IP address against rate limit
		clientIP := clientAddr.IP.String()
		if !daemon.rateLimit.Add(clientIP, true) {
			continue
		}
		go func(clientAddr *net.UDPAddr) {
			// Put processing duration (including IO time) into statistics
			beginTimeNano := time.Now().UnixNano()
			defer func() {
				common.SimpleIPStatsUDP.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
			}()
			daemon.logger.Info(logFunName, clientIP, nil, "working on the request")
			udpServer.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second))
			if _, err := udpServer.WriteToUDP([]byte(daemon.serverResponseFun[port]()), clientAddr); err != nil {
				daemon.logger.Warning(logFunName, clientIP, err, "failed to write response")
			}
		}(clientAddr)
	}
}

func TestSimpleIPSvcD(daemon *Daemon, t testingstub.T) {
	if os.Getuid() != 0 {
		t.Log("skipped simple IP service tests due to lack of root privilege")
		return
	}

	// Server should start within two seconds
	var stoppedNormally bool
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stoppedNormally = true
	}()
	time.Sleep(2 * time.Second)

	// The function returns true only if the response matches expectation from the service
	testResponseMatch := func(port int, response string) bool {
		switch port {
		case 11:
			return strings.TrimSpace(response) == daemon.ActiveUserNames
		case 12 + 1:
			// No need to match minute and second
			return strings.Contains(response, time.Now().Format("2006-01-02T15"))
		case 17:
			return strings.TrimSpace(response) == daemon.QOTD
		}
		return false
	}

	// Pick port 11 as the test subject for rate limits, TCP and UDP.
	success := 0
	for i := 0; i < 40; i++ {
		tcpClient, err := net.Dial("tcp", "127.0.0.1:11")
		if err != nil {
			continue
		}
		tcpClient.SetDeadline(time.Now().Add(50 * time.Millisecond))
		response, err := ioutil.ReadAll(tcpClient)
		if err != nil {
			continue
		}
		tcpClient.Close()
		if testResponseMatch(11, string(response)) {
			success++
		}
	}
	if success < 1 || success > daemon.PerIPLimit*2 {
		t.Fatal("number of succeeded TCP requests is wrong", success)
	}
	// Wait out rate limit (leave 3 seconds buffer for pending requests to complete)
	time.Sleep((RateLimitIntervalSec + 3) * time.Second)
	success = 0
	for i := 0; i < 40; i++ {
		udpClient, err := net.Dial("udp", "127.0.0.1:11")
		if err != nil {
			continue
		}
		udpClient.SetDeadline(time.Now().Add(50 * time.Millisecond))
		_, err = udpClient.Write([]byte{})
		if err != nil {
			continue
		}
		udpResponse, err := bufio.NewReader(udpClient).ReadString('\n')
		if err != nil {
			continue
		}
		udpClient.Close()
		if testResponseMatch(11, udpResponse) {
			success++
		}
	}
	if success < 1 || success > daemon.PerIPLimit*2 {
		t.Fatal("number of succeeded UDP requests is wrong", success)
	}
	// Wait out rate limit (leave 3 seconds buffer for pending requests to complete)
	time.Sleep((RateLimitIntervalSec + 3) * time.Second)

	// Test each of the three services
	for _, port := range []int{11, 12 + 1, 17} {
		// Test TCP implementation of the service
		tcpClient, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			t.Fatal(err)
		}
		response, err := ioutil.ReadAll(tcpClient)
		if err != nil {
			t.Fatal(err)
		}
		tcpClient.Close()
		if !testResponseMatch(port, string(response)) {
			t.Fatal(port, string(response))
		}
		// Test UDP implementation of the service
		udpClient, err := net.Dial("udp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			t.Fatal(err)
		}
		_, err = udpClient.Write([]byte{})
		if err != nil {
			t.Fatal(err)
		}
		udpResponse, err := bufio.NewReader(udpClient).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		udpClient.Close()
		if !testResponseMatch(port, udpResponse) {
			t.Fatal(port, udpResponse)
		}
	}

	// Daemon should stop within a second
	daemon.Stop()
	time.Sleep(1 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}

	// Repeatedly stopping the daemon should have no negative consequence
	daemon.Stop()
	daemon.Stop()
	daemon.Stop()
}
