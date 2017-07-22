package dnsd

import (
	"bytes"
	"fmt"
	"github.com/HouzuoGuo/laitos/env"
	"github.com/HouzuoGuo/laitos/global"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

var UDPDurationStats = env.NewStats() // UDPDurationStats stores statistics of duration of all UDP DNS queries.

// Send forward queries to forwarder and forward the response to my DNS client.
func (dnsd *DNSD) HandleUDPQueries(myQueue chan *UDPQuery, forwarderConn net.Conn) {
	packetBuf := make([]byte, MaxPacketSize)
	for {
		query := <-myQueue
		// Put query duration (including IO time) into statistics
		beginTimeNano := time.Now().UnixNano()
		// Set deadline for IO with forwarder
		forwarderConn.SetDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		if _, err := forwarderConn.Write(query.QueryPacket); err != nil {
			dnsd.Logger.Warningf("HandleUDPQueries", query.ClientAddr.String(), err, "failed to write to forwarder")
			UDPDurationStats.Trigger(float64((time.Now().UnixNano() - beginTimeNano) / 1000000))
			continue
		}
		packetLength, err := forwarderConn.Read(packetBuf)
		if err != nil {
			dnsd.Logger.Warningf("HandleUDPQueries", query.ClientAddr.String(), err, "failed to read from forwarder")
			UDPDurationStats.Trigger(float64((time.Now().UnixNano() - beginTimeNano) / 1000000))
			continue
		}
		// Set deadline for responding to my DNS client
		query.MyServer.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		if _, err := query.MyServer.WriteTo(packetBuf[:packetLength], query.ClientAddr); err != nil {
			dnsd.Logger.Warningf("HandleUDPQueries", query.ClientAddr.String(), err, "failed to answer to client")
			UDPDurationStats.Trigger(float64((time.Now().UnixNano() - beginTimeNano) / 1000000))
			continue
		}
	}
}

// Send blackhole answer to my DNS client.
func (dnsd *DNSD) HandleBlackHoleAnswer(myQueue chan *UDPQuery) {
	for {
		query := <-myQueue
		// Put query duration (including IO time) into statistics
		beginTimeNano := time.Now().UnixNano()
		// Set deadline for responding to my DNS client
		blackHoleAnswer := RespondWith0(query.QueryPacket)
		query.MyServer.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		if _, err := query.MyServer.WriteTo(blackHoleAnswer, query.ClientAddr); err != nil {
			dnsd.Logger.Warningf("HandleUDPQueries", query.ClientAddr.String(), err, "IO failure")
		}
		UDPDurationStats.Trigger(float64((time.Now().UnixNano() - beginTimeNano) / 1000000))
	}
}

/*
You may call this function only after having called Initialise()!
Start DNS daemon to listen on UDP port only, until daemon is told to stop.
*/
func (dnsd *DNSD) StartAndBlockUDP() error {
	listenAddr := fmt.Sprintf("%s:%d", dnsd.Address, dnsd.UDPPort)
	udpAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return err
	}
	udpServer, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	defer udpServer.Close()
	dnsd.UDPListener = udpServer
	dnsd.Logger.Printf("StartAndBlockUDP", listenAddr, nil, "going to listen for queries")
	// Start queues that will respond to DNS clients
	for i, queue := range dnsd.UDPForwarderQueues {
		go dnsd.HandleUDPQueries(queue, dnsd.UDPForwarderConns[i])
	}
	for _, queue := range dnsd.UDPBlackHoleQueues {
		go dnsd.HandleBlackHoleAnswer(queue)
	}
	// Dispatch queries to forwarder queues
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
			return fmt.Errorf("DNSD.StartAndBlockUDP: failed to accept new connection - %v", err)
		}
		// Check address against rate limit and allowed IP prefixes
		clientIP := clientAddr.IP.String()
		if !dnsd.RateLimit.Add(clientIP, true) {
			continue
		}
		if !dnsd.checkAllowClientIP(clientIP) {
			dnsd.Logger.Warningf("UDPLoop", clientIP, nil, "client IP is not allowed to query")
			continue
		}

		// Prepare parameters for forwarding the query
		randForwarder := rand.Intn(len(dnsd.UDPForwarderQueues))
		forwardPacket := make([]byte, packetLength)
		copy(forwardPacket, packetBuf[:packetLength])
		domainName := ExtractDomainName(forwardPacket)
		if len(domainName) == 0 {
			// If I cannot figure out what domain is from the query, simply forward it without much concern.
			dnsd.Logger.Printf(fmt.Sprintf("UDP-%d", randForwarder), clientIP, nil,
				"handle non-name query (backlog %d)", len(dnsd.UDPForwarderQueues[randForwarder]))
			dnsd.UDPForwarderQueues[randForwarder] <- &UDPQuery{
				ClientAddr:  clientAddr,
				MyServer:    udpServer,
				QueryPacket: forwardPacket,
			}
		} else if dnsd.NamesAreBlackListed(domainName) {
			// Requested domain name is black-listed
			randBlackListResponder := rand.Intn(len(dnsd.UDPBlackHoleQueues))
			dnsd.Logger.Printf(fmt.Sprintf("UDP-%d", randBlackListResponder), clientIP, nil,
				"handle black-listed domain \"%s\" (backlog %d)", domainName[0], len(dnsd.UDPBlackHoleQueues[randBlackListResponder]))
			dnsd.UDPBlackHoleQueues[randBlackListResponder] <- &UDPQuery{
				ClientAddr:  clientAddr,
				MyServer:    udpServer,
				QueryPacket: forwardPacket,
			}
		} else {
			// This is a normal domain name query and not black-listed
			dnsd.Logger.Printf(fmt.Sprintf("UDP-%d", randForwarder), clientIP, nil,
				"handle domain \"%s\" (backlog %d)", domainName[0], len(dnsd.UDPForwarderQueues[randForwarder]))
			dnsd.UDPForwarderQueues[randForwarder] <- &UDPQuery{
				ClientAddr:  clientAddr,
				MyServer:    udpServer,
				QueryPacket: forwardPacket,
			}
		}
	}
}

// Run unit tests on DNS UDP daemon. See TestDNSD_StartAndBlockUDP for daemon setup.
func TestUDPQueries(dnsd *DNSD, t *testing.T) {
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
	// Try to reach rate limit
	var success int
	for i := 0; i < 40; i++ {
		go func() {
			clientConn, err := net.DialUDP("udp", nil, serverAddr)
			if err != nil {
				t.Fatal(err)
			}
			defer clientConn.Close()
			if err := clientConn.SetDeadline(time.Now().Add((RateLimitIntervalSec - 1) * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := clientConn.Write(githubComUDPQuery); err != nil {
				t.Fatal(err)
			}
			length, err := clientConn.Read(packetBuf)
			if err == nil && length > 50 {
				success++
			}
		}()
	}
	// Wait out rate limit
	time.Sleep(RateLimitIntervalSec * time.Second)
	if success < 5 || success > 15 {
		t.Fatal(success)
	}
	// Blacklist github and see if query gets a black hole response
	dnsd.BlackList["github.com"] = struct{}{}
	// This test is flaky and I do not understand why, is it throttled by google dns?
	var blackListSuccess bool
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		clientConn, err := net.DialUDP("udp", nil, serverAddr)
		if err != nil {
			continue
		}
		if err := clientConn.SetDeadline(time.Now().Add((RateLimitIntervalSec - 1) * time.Second)); err != nil {
			continue
			clientConn.Close()
		}
		if _, err := clientConn.Write(githubComUDPQuery); err != nil {
			continue
			clientConn.Close()
		}
		respLen, err := clientConn.Read(packetBuf)
		if err != nil {
			continue
			clientConn.Close()
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
	time.Sleep(10 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	dnsd.Stop()
	dnsd.Stop()
}
