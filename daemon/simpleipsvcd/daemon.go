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
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/testingstub"
)

const (
	IOTimeoutSec         = 30 // IOTimeoutSec is the timeout used in network request/response operations.
	RateLimitIntervalSec = 1  // RateLimitIntervalSec is the interval for rate limit calculation.
)

// Daemon implements simple & standard Internet services that were used in the nostalgic era of computing.
type Daemon struct {
	Address         string `json:"Address"`         // Address to listen on, e.g. 0.0.0.0 to listen on all network interfaces.
	ActiveUsersPort int    `json:"ActiveUsersPort"` // ActiveUsersPort is the port number (TCP and UDP) to listen on for the sysstat (active user names) service.
	DayTimePort     int    `json:"DayTimePort"`     // DayTimePort is the port number (TCP and UDP) to listen on for the daytime service.
	QOTDPort        int    `json:"QOTDPort"`        // QOTDPort is the port number (TCP and UDP) to listen on for the QOTD service.
	PerIPLimit      int    `json:"PerIPLimit"`      // PerIPLimit is approximately how many requests are allowed from an IP within a designated interval.
	ActiveUserNames string `json:"ActiveUserNames"` // ActiveUserNames are CRLF-separated list of user names to appear in the response of "sysstat" network service.
	QOTD            string `json:"QOTD"`            // QOTD is the message to appear in the response of "QOTD" network service.

	logger lalog.Logger

	// tcpServers, udpServers, and serverResponseFun contain server instances and their corresponding response content function.
	tcpServers        map[int]*common.TCPServer
	udpServers        map[int]*common.UDPServer
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
	if daemon.ActiveUsersPort < 1 {
		daemon.ActiveUsersPort = 11
	}
	if daemon.DayTimePort < 1 {
		daemon.DayTimePort = 12 + 1
	}
	if daemon.QOTDPort < 1 {
		daemon.QOTDPort = 17
	}
	daemon.ActiveUserNames = strings.TrimSpace(daemon.ActiveUserNames)
	daemon.QOTD = strings.TrimSpace(daemon.QOTD)

	daemon.logger = lalog.Logger{
		ComponentName: "simpleipsvcd",
		ComponentID:   []lalog.LoggerIDField{{Key: "Addr", Value: daemon.Address}},
	}

	daemon.tcpServers = make(map[int]*common.TCPServer)
	daemon.udpServers = make(map[int]*common.UDPServer)
	daemon.serverResponseFun = map[int]func() string{
		daemon.ActiveUsersPort: daemon.responseActiveUsers,
		daemon.DayTimePort:     daemon.responseDayTime,
		daemon.QOTDPort:        daemon.responseQOTD,
	}
	return nil
}

// StartAndBlock starts all TCP and UDP servers to serve network clients. You may call this function only after having called Initialise().
func (daemon *Daemon) StartAndBlock() error {
	defer daemon.Stop()
	// There are 3 TCP servers and 3 UDP servers
	wg := new(sync.WaitGroup)
	for _, port := range []int{daemon.ActiveUsersPort, daemon.DayTimePort, daemon.QOTDPort} {
		// There is one TCP server and one UDP server per daemon
		wg.Add(2)
		daemon.logger.Info("StartAndBlock", "", nil, "going to listen on TCP and UDP port %d", port)
		// Start TCP listener on the port
		tcpServer := &common.TCPServer{
			ListenAddr:  daemon.Address,
			ListenPort:  port,
			AppName:     "simpleipsvc",
			App:         &TCPService{ResponseFun: daemon.serverResponseFun[port]},
			LimitPerSec: daemon.PerIPLimit,
		}
		tcpServer.Initialise()
		daemon.tcpServers[port] = tcpServer
		go func(tcpServer *common.TCPServer) {
			defer wg.Done()
			if err := tcpServer.StartAndBlock(); err != nil {
				daemon.logger.Warning("StartAndBlock", strconv.Itoa(tcpServer.ListenPort), err, "failed to start a TCP server")
			}
		}(tcpServer)

		// Start UDP server on the port
		udpServer := &common.UDPServer{
			ListenAddr:  daemon.Address,
			ListenPort:  port,
			AppName:     "simpleipsvc",
			App:         &UDPService{ResponseFun: daemon.serverResponseFun[port]},
			LimitPerSec: daemon.PerIPLimit,
		}
		udpServer.Initialise()
		daemon.udpServers[port] = udpServer
		go func(udpServer *common.UDPServer) {
			defer wg.Done()
			if err := udpServer.StartAndBlock(); err != nil {
				daemon.logger.Warning("StartAndBlock", strconv.Itoa(udpServer.ListenPort), err, "failed to start a UDP server")
			}
		}(udpServer)
	}
	// Wait for servers to stop
	wg.Wait()
	return nil
}

// Stop terminates all TCP and UDP servers.
func (daemon *Daemon) Stop() {
	if daemon.tcpServers != nil {
		for _, tcpDaemon := range daemon.tcpServers {
			if tcpDaemon != nil {
				tcpDaemon.Stop()
			}
		}
	}
	if daemon.udpServers != nil {
		for _, udpDaemon := range daemon.udpServers {
			if udpDaemon != nil {
				udpDaemon.Stop()
			}
		}
	}
	daemon.tcpServers = make(map[int]*common.TCPServer)
	daemon.udpServers = make(map[int]*common.UDPServer)
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

func TestSimpleIPSvcD(daemon *Daemon, t testingstub.T) {
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
		case daemon.ActiveUsersPort:
			return strings.TrimSpace(response) == daemon.ActiveUserNames
		case daemon.DayTimePort:
			// No need to match minute and second
			return strings.Contains(response, time.Now().Format("2006-01-02T15"))
		case daemon.QOTDPort:
			return strings.TrimSpace(response) == daemon.QOTD
		}
		return false
	}

	// Test each of the three services
	for _, port := range []int{daemon.ActiveUsersPort, daemon.DayTimePort, daemon.QOTDPort} {
		// Test TCP implementation of the service
		tcpClient, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			t.Fatal(err)
		}
		response, err := ioutil.ReadAll(tcpClient)
		if err != nil {
			t.Fatal(err)
		}
		_ = tcpClient.Close()
		if !testResponseMatch(port, string(response)) {
			t.Fatal(port, string(response))
		}
		// Test UDP implementation of the service
		udpClient, err := net.Dial("udp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			t.Fatal(err)
		}
		_, err = udpClient.Write([]byte{0})
		if err != nil {
			t.Fatal(err)
		}
		udpResponse, err := bufio.NewReader(udpClient).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		_ = udpClient.Close()
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
