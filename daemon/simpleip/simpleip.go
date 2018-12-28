/*
simpleip implements simple & standard Internet services that were used in the nostalgic era of computing.
According to those protocol standard, the services are available via TCP and UDP simultaneously:
- sysstat (active system user name on port 11, RFC 866)
- daytime (current system time in readable text on port 12+1, RFC 867)
- QOTD (short text message as quote of the day on port 17, RFC 865)
*/
package simpleip

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

const (
	IOTimeoutSec         = 60 // IOTimeoutSec is the number of seconds to tolerate for network IO operations.
	RateLimitIntervalSec = 1  // RateLimitIntervalSec is the interval for rate limit calculation.
)

// Daemon implements simple & standard Internet services that were used in the nostalgic era of computing.
type Daemon struct {
	Address         string `json:"Address"`        // Address to listen on, e.g. 0.0.0.0 to listen on all network interfaces.
	PerIPLimit      int    `json:"PerIPLimit"`     // PerIPLimit is approximately how many requests are allowed from an IP within a designated interval.
	ActiveUserNames string `json:"ActiveUserName"` // ActiveUserNames are LF-separated list of user names to appear in the response of "sysstat" network service.
	QOTD            string `json:"QOTD"`           // QOTD is the message to appear in the response of "QOTD" network service.

	rateLimit *misc.RateLimit
	logger    lalog.Logger

	/*
		tcpServers, udpServers, and serverResponseFun contain listener structures and response content functions in the following order:
		0. active users on port 11
		1. daytime on port 12+1
		2. QOTD on port 17
	*/

	tcpServers        []net.Listener
	udpServers        []*net.UDPConn
	serverResponseFun []func() string
}

// Initialise validates configuration and initialises internal states.
func (daemon *Daemon) Initialise() error {
	if daemon.Address == "" {
		daemon.Address = "0.0.0.0"
	}
	if daemon.PerIPLimit < 1 {
		daemon.PerIPLimit = 3 // more than sufficient for those protocol use cases
	}
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
	// Initialise array of server-side structures for total of 3 simple IP services
	daemon.tcpServers = make([]net.Listener, 3)
	daemon.udpServers = make([]*net.UDPConn, 3)
	daemon.serverResponseFun = []func() string{daemon.responseActiveUsers, daemon.responseDayTime, daemon.responseQOTD}
	return nil
}

// StartAndBlock starts all TCP and UDP servers to serve network clients. You may call this function only after having called Initialise().
func (daemon *Daemon) StartAndBlock() error {
	// There are 3 TCP servers and 3 UDP servers
	wg := new(sync.WaitGroup)
	wg.Add(6)
	// 11 - active users; 12+1 - daytime; 17 - QOTD
	for i, port := range []int{11, 12 + 1, 17} {
		// Start TCP listener on the port
		tcpServer, err := net.Listen("tcp", fmt.Sprintf("%s:%d", daemon.Address, port))
		if err != nil {
			daemon.Stop()
			return fmt.Errorf("simpleip.StartAndBlock: failed to start TCP server on port %d - %v", port, err)
		}
		daemon.tcpServers[i] = tcpServer
		go func(i int) {
			daemon.tcpResponderLoop(daemon.tcpServers[i], daemon.serverResponseFun[i])
			wg.Done()
		}(i)

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
		daemon.udpServers[i] = udpServer
		go func(i int) {
			daemon.udpResponderLoop(daemon.udpServers[i], daemon.serverResponseFun[i])
			wg.Done()
		}(i)
	}
	// Wait for servers to be stop
	wg.Wait()
	return nil
}

// Stop terminates all TCP and UDP servers.
func (daemon *Daemon) Stop() {
	if daemon.tcpServers != nil {
		for i, server := range daemon.tcpServers {
			if server == nil {
				continue
			}
			if err := server.Close(); err != nil {
				daemon.logger.Warning("Stop", "", err, "failed to stop a TCP server")
			}
			daemon.tcpServers[i] = nil
		}
	}
	if daemon.udpServers != nil {
		for i, server := range daemon.udpServers {
			if server == nil {
				continue
			}
			if err := server.Close(); err != nil {
				daemon.logger.Warning("Stop", "", err, "failed to stop a UDP server")
			}
			daemon.udpServers[i] = nil
		}
	}
}

// responseActiveUsers returns configured active system user names in response to a sysstat service client.
func (daemon *Daemon) responseActiveUsers() string {
	return daemon.ActiveUserNames
}

// responseDayTime returns the current system time inn RFC3339 format in response to a daytime service client.
func (daemon *Daemon) responseDayTime() string {
	return time.Now().Format(time.RFC3339)
}

// responseQOTD returns the configured QOTD in response to a QOTD service client.
func (daemon *Daemon) responseQOTD() string {
	return daemon.QOTD
}

// tcpResponderLoop blocks caller to server all incoming TCP connections until the listener is closed.
func (daemon *Daemon) tcpResponderLoop(listener net.Listener, contentFun func() string) {
}

// udpResponderLoop blocks caller to serve all incoming UDP requests until the server is closed.
func (daemon *Daemon) udpResponderLoop(udpServer *net.UDPConn, contentFun func() string) {
}
