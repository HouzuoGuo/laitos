package dnsd

import (
	"bytes"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
)

/*
StartAndBlockUDP starts DNS daemon to listen on UDP port only, until daemon is told to stop.
Daemon must have already been initialised prior to this call.
*/
func (daemon *Daemon) StartAndBlockUDP() error {
	listenAddr := net.JoinHostPort(daemon.Address, strconv.Itoa(daemon.UDPPort))
	udpAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return err
	}
	udpServer, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	defer udpServer.Close()
	daemon.udpListener = udpServer
	daemon.logger.Info("StartAndBlockUDP", listenAddr, nil, "going to listen for queries")
	// Dispatch queries to forwarder queues
	packetBuf := make([]byte, MaxPacketSize)
	for {
		if misc.EmergencyLockDown {
			return misc.ErrEmergencyLockDown
		}
		packetLength, clientAddr, err := udpServer.ReadFromUDP(packetBuf)
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			return fmt.Errorf("DNSD.StartAndBlockUDP: failed to accept new connection - %v", err)
		}
		// Check address against rate limit and allowed IP prefixes
		clientIP := clientAddr.IP.String()
		if !daemon.rateLimit.Add(clientIP, true) {
			continue
		}
		if !daemon.checkAllowClientIP(clientIP) {
			daemon.logger.Warning("UDPLoop", clientIP, nil, "client IP is not allowed to query")
			continue
		}
		if packetLength < 3 {
			daemon.logger.Warning("UDPLoop", clientIP, nil, "received packet is abnormally small")
			continue
		}
		queryPacket := make([]byte, packetLength)
		copy(queryPacket, packetBuf[:packetLength])
		go daemon.handleUDPQuery(queryPacket, clientAddr)
	}
}

func (daemon *Daemon) handleUDPQuery(queryBody []byte, client *net.UDPAddr) {
	// Put query duration (including IO time) into statistics
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		common.DNSDStatsUDP.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()
	clientIP := client.IP.String()

	var respLenInt int
	var respBody []byte
	if isTextQuery(queryBody) {
		// Handle toolbox command that arrives as a text query
		respBody, respLenInt = daemon.handleUDPTextQuery(clientIP, queryBody)
	} else {
		// Handle other query types such as name query
		respBody, respLenInt = daemon.handleUDPNameOrOtherQuery(clientIP, queryBody)
	}
	// Send response to the client
	// Set deadline for responding to my DNS client because the query reader and response writer do not share the same timeout
	daemon.udpListener.SetWriteDeadline(time.Now().Add(ClientTimeoutSec * time.Second))
	if _, err := daemon.udpListener.WriteTo(respBody[:respLenInt], client); err != nil {
		daemon.logger.Warning("HandleUDPQuery", clientIP, err, "failed to answer to client")
		return
	}
}

func (daemon *Daemon) handleUDPTextQuery(clientIP string, queryBody []byte) (resp []byte, respLenInt int) {
	resp = make([]byte, 0)
	return daemon.handleUDPNameOrOtherQuery(clientIP, queryBody)
	// TODO: implement this
}

func (daemon *Daemon) handleUDPNameOrOtherQuery(clientIP string, queryBody []byte) (resp []byte, respLenInt int) {
	resp = make([]byte, 0)
	// Handle other query types such as name query
	domainName := ExtractDomainName(queryBody)
	if domainName == "" {
		daemon.logger.Info("handleUDPNameOrOtherQuery", clientIP, nil, "handle non-name query")
	} else {
		daemon.logger.Info("handleUDPNameOrOtherQuery", clientIP, nil, "handle query \"%s\"", domainName)
	}
	if daemon.IsInBlacklist(domainName) {
		// Formulate a black-hole response to black-listed domain name
		daemon.logger.Info("handleUDPNameOrOtherQuery", clientIP, nil, "handle black-listed \"%s\"", domainName)
		resp = GetBlackHoleResponse(queryBody)
		respLenInt = len(resp)
		return
	}
	return daemon.handleUDPRecursiveQuery(clientIP, queryBody)
}

func (daemon *Daemon) handleUDPRecursiveQuery(clientIP string, queryBody []byte) (resp []byte, respLenInt int) {
	resp = make([]byte, 0)
	// Forward the query to a randomly chosen recursive resolver and return its response
	randForwarder := daemon.Forwarders[rand.Intn(len(daemon.Forwarders))]
	forwarderConn, err := net.DialTimeout("udp", randForwarder, ForwarderTimeoutSec*time.Second)
	if err != nil {
		daemon.logger.Warning("handleUDPRecursiveQuery", clientIP, err, "failed to dial forwarder's address")
		return
	}
	forwarderConn.SetDeadline(time.Now().Add(ForwarderTimeoutSec * time.Second))
	if _, err := forwarderConn.Write(queryBody); err != nil {
		daemon.logger.Warning("handleUDPRecursiveQuery", clientIP, err, "failed to write to forwarder")
		return
	}
	resp = make([]byte, MaxPacketSize)
	respLenInt, err = forwarderConn.Read(resp)
	if err != nil {
		daemon.logger.Warning("handleUDPRecursiveQuery", clientIP, err, "failed to read from forwarder")
		return
	}
	if respLenInt < 3 {
		daemon.logger.Warning("handleUDPRecursiveQuery", clientIP, err, "forwarder response is abnormally small")
		return
	}
	return
}

// Run unit tests on DNS UDP daemon. See TestDNSD for daemon setup.
func TestUDPQueries(dnsd *Daemon, t testingstub.T) {
	if misc.HostIsWindows() {
		// FIXME: fix this test case for Windows
		t.Log("FIXME: enable TestUDPQueries for Windows")
		return
	}
	// Prevent daemon from listening to TCP queries in this UDP test case
	tcpListenPort := dnsd.TCPPort
	dnsd.TCPPort = 0
	defer func() {
		dnsd.TCPPort = tcpListenPort
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

	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+strconv.Itoa(dnsd.UDPPort))
	if err != nil {
		t.Fatal(err)
	}
	packetBuf := make([]byte, MaxPacketSize)

	oldBlacklist := dnsd.blackList
	defer func() {
		dnsd.blackList = oldBlacklist
	}()

	// Try to reach rate limit - assume rate limit is 10
	var success int
	for i := 0; i < 40; i++ {
		go func() {
			clientConn, err := net.DialUDP("udp", nil, serverAddr)
			if err != nil {
				t.Fatal(err)
			}
			defer clientConn.Close()
			if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := clientConn.Write(GithubComUDPQuery); err != nil {
				t.Fatal(err)
			}
			length, err := clientConn.Read(packetBuf)
			if err == nil && length > 50 {
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
		clientConn, err := net.DialUDP("udp", nil, serverAddr)
		if err != nil {
			continue
		}
		if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			clientConn.Close()
			continue
		}
		if _, err := clientConn.Write(GithubComUDPQuery); err != nil {
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
