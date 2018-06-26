package dnsd

import (
	"bytes"
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
	"io/ioutil"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

var TCPDurationStats = misc.NewStats() // TCPDurationStats stores statistics of duration of all TCP DNS queries.

func (daemon *Daemon) handleTCPQuery(clientConn net.Conn) {
	// Put query duration (including IO time) into statistics
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		TCPDurationStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()
	defer clientConn.Close()
	clientIP := clientConn.RemoteAddr().(*net.TCPAddr).IP.String()
	// Read query length
	clientConn.SetDeadline(time.Now().Add(ClientTimeoutSec * time.Second))
	queryLenBuf := make([]byte, 2)
	_, err := clientConn.Read(queryLenBuf)
	if err != nil {
		daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to read query length from client")
		return
	}
	queryLen := int(queryLenBuf[0])*256 + int(queryLenBuf[1])
	// Read query
	if queryLen > MaxPacketSize || queryLen < 3 {
		daemon.logger.Warning("handleTCPQuery", clientIP, nil, "invalid query length from client")
		return
	}
	queryBuf := make([]byte, queryLen)
	_, err = clientConn.Read(queryBuf)
	if err != nil {
		daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to read query from client")
		return
	}
	// Prepare for a response
	domainName := ExtractDomainName(queryBuf)
	var forwarderRespLen int
	var forwardRespLenBuf []byte
	var forwarderResp []byte
	if daemon.IsInBlacklist(domainName) {
		// Formulate a black-hole response
		daemon.logger.Info("handleTCPQuery", clientIP, nil, "handle black-listed \"%s\"", domainName)
		forwarderResp = RespondWith0(queryBuf)
		forwarderRespLen = len(forwarderResp)
		forwardRespLenBuf = make([]byte, 2)
		forwardRespLenBuf[0] = byte(forwarderRespLen / 256)
		forwardRespLenBuf[1] = byte(forwarderRespLen % 256)
	} else {
		randForwarder := daemon.Forwarders[rand.Intn(len(daemon.Forwarders))]
		if domainName == "" {
			daemon.logger.Info("handleTCPQuery", clientIP, nil, "handle non-name query via %s", randForwarder)
		} else {
			daemon.logger.Info("handleTCPQuery", clientIP, nil, "handle query \"%s\" via %s", domainName, randForwarder)
		}
		// Ask a randomly chosen TCP forwarder to process the query
		myForwarder, err := net.DialTimeout("tcp", randForwarder, ForwarderTimeoutSec*time.Second)
		if err != nil {
			daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to connect to forwarder")
			return
		}
		defer myForwarder.Close()
		// Send original query to forwarder without modification
		myForwarder.SetDeadline(time.Now().Add(ForwarderTimeoutSec * time.Second))
		if _, err = myForwarder.Write(queryLenBuf); err != nil {
			daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to write length to forwarder")
			return
		} else if _, err = myForwarder.Write(queryBuf); err != nil {
			daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to write query to forwarder")
			return
		}
		// Retrieve forwarder's response
		forwardRespLenBuf = make([]byte, 2)
		if _, err = myForwarder.Read(forwardRespLenBuf); err != nil {
			daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to read length from forwarder")
			return
		}
		forwarderRespLen = int(forwardRespLenBuf[0])*256 + int(forwardRespLenBuf[1])
		if forwarderRespLen > MaxPacketSize || forwarderRespLen < 1 {
			daemon.logger.Warning("handleTCPQuery", clientIP, nil, "bad response length from forwarder")
			return
		}
		forwarderResp = make([]byte, forwarderRespLen)
		if _, err = myForwarder.Read(forwarderResp); err != nil {
			daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to read response from forwarder")
			return
		}
	}
	// Send response to my client
	if _, err = clientConn.Write(forwardRespLenBuf); err != nil {
		daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to answer length to client")
		return
	} else if _, err = clientConn.Write(forwarderResp); err != nil {
		daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to answer to client")
		return
	}
	return
}

/*
You may call this function only after having called Initialise()!
Start DNS daemon to listen on TCP port only, until daemon is told to stop.
*/
func (daemon *Daemon) StartAndBlockTCP() error {
	listenAddr := net.JoinHostPort(daemon.Address, strconv.Itoa(daemon.TCPPort))
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	defer listener.Close()
	daemon.tcpListener = listener
	// Process incoming TCP DNS queries
	daemon.logger.Info("StartAndBlockTCP", listenAddr, nil, "going to listen for queries")
	for {
		if misc.EmergencyLockDown {
			return misc.ErrEmergencyLockDown
		}
		clientConn, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			return fmt.Errorf("DNSD.StartAndBlockTCP: failed to accept new connection - %v", err)
		}
		// Check address against rate limit and allowed IP prefixes
		clientIP := clientConn.RemoteAddr().(*net.TCPAddr).IP.String()
		if !daemon.rateLimit.Add(clientIP, true) {
			clientConn.Close()
			continue
		}
		if !daemon.checkAllowClientIP(clientIP) {
			daemon.logger.Warning("StartAndBlockTCP", clientIP, nil, "client IP is not allowed to query")
			clientConn.Close()
			continue
		}
		go daemon.handleTCPQuery(clientConn)
	}

}

// Run unit tests on DNS TCP daemon. See TestDNSD_StartAndBlockTCP for daemon setup.
func TestTCPQueries(dnsd *Daemon, t testingstub.T) {
	if misc.HostIsWindows() {
		// FIXME: fix this test case for Windows
		t.Skip("FIXME: enable this test case for Windows")
	}
	// Prevent daemon from listening to UDP queries in this TCP test case
	dnsd.UDPPort = 0
	udpListenPort := dnsd.UDPPort
	dnsd.UDPPort = 0
	defer func() {
		dnsd.UDPPort = udpListenPort
	}()
	// Server should start within two seconds
	var stoppedNormally bool
	go func() {
		if err := dnsd.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stoppedNormally = true
	}()
	time.Sleep(2 * time.Second)

	oldBlacklist := dnsd.blackList
	defer func() {
		dnsd.blackList = oldBlacklist
	}()

	packetBuf := make([]byte, MaxPacketSize)
	// Try to reach rate limit - assume rate limit is 10
	success := 0
	dnsd.blackList = map[string]struct{}{}
	for i := 0; i < 40; i++ {
		go func() {
			clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(dnsd.TCPPort))
			if err != nil {
				t.Fatal(err)
			}
			defer clientConn.Close()
			if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := clientConn.Write(GithubComTCPQuery); err != nil {
				t.Fatal(err)
			}
			resp, err := ioutil.ReadAll(clientConn)
			if err == nil && len(resp) > 50 {
				success++
			}
		}()
	}
	// Wait out rate limit (leave 3 seconds buffer for pending requests to complete)
	time.Sleep((RateLimitIntervalSec + 3) * time.Second)
	if success < 1 || success > dnsd.PerIPLimit*2 {
		t.Fatal(success)
	}
	// Blacklist github and see if query gets a black hole response
	dnsd.blackList["github.com"] = struct{}{}
	// This test is flaky and I do not understand why, is it throttled by google dns?
	var blackListSuccess bool
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(dnsd.TCPPort))
		if err != nil {
			continue
		}
		if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			clientConn.Close()
			continue
		}
		if _, err := clientConn.Write(GithubComTCPQuery); err != nil {
			clientConn.Close()
			continue
		}
		respLen, err := clientConn.Read(packetBuf)
		if err != nil {
			clientConn.Close()
			continue
		}
		clientConn.Close()
		if bytes.Index(packetBuf[:respLen], BlackHoleAnswer) != -1 {
			blackListSuccess = true
			break
		}
	}
	if !blackListSuccess {
		t.Fatal("did not answer to blacklist domain")
	}
	// Daemon must stop in a second
	dnsd.Stop()
	time.Sleep(1 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	dnsd.Stop()
	dnsd.Stop()
}
