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

func (daemon *Daemon) HandleTCPQuery(clientConn net.Conn) {
	// Put query duration (including IO time) into statistics
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		TCPDurationStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()
	defer clientConn.Close()
	// Check address against rate limit and allowed IP prefixes
	clientIP := clientConn.RemoteAddr().(*net.TCPAddr).IP.String()
	if !daemon.rateLimit.Add(clientIP, true) {
		return
	}
	if !daemon.checkAllowClientIP(clientIP) {
		daemon.logger.Warningf("HandleTCPQuery", clientIP, nil, "client IP is not allowed to query")
		return
	}
	// Read query length
	clientConn.SetDeadline(time.Now().Add(IOTimeoutSec * time.Second))
	queryLenBuf := make([]byte, 2)
	_, err := clientConn.Read(queryLenBuf)
	if err != nil {
		daemon.logger.Warningf("HandleTCPQuery", clientIP, err, "failed to read query length from client")
		return
	}
	queryLen := int(queryLenBuf[0])*256 + int(queryLenBuf[1])
	// Read query
	if queryLen > MaxPacketSize || queryLen < 1 {
		daemon.logger.Warningf("HandleTCPQuery", clientIP, nil, "bad query length from client")
		return
	}
	queryBuf := make([]byte, queryLen)
	_, err = clientConn.Read(queryBuf)
	if err != nil {
		daemon.logger.Warningf("HandleTCPQuery", clientIP, err, "failed to read query from client")
		return
	}
	// Parse request and formulate a response
	requestedDomainName := ExtractDomainName(queryBuf)
	var responseLen int
	var responseLenBuf []byte
	var responseBuf []byte
	var doForward bool
	if requestedDomainName == "" {
		// If I cannot figure out what domain is from the query, simply forward it without much concern.
		daemon.logger.Printf("HandleTCPQuery", clientIP, nil, "handle non-name query")
		doForward = true
	} else {
		// This is a domain name query, check the name against black list and then forward.
		if daemon.IsInBlacklist(requestedDomainName) {
			daemon.logger.Printf("HandleTCPQuery", clientIP, nil, "handle black-listed domain \"%s\"", requestedDomainName)
			responseBuf = RespondWith0(queryBuf)
			responseLen = len(responseBuf)
			responseLenBuf = make([]byte, 2)
			responseLenBuf[0] = byte(responseLen / 256)
			responseLenBuf[1] = byte(responseLen % 256)
		} else {
			daemon.logger.Printf("HandleTCPQuery", clientIP, nil, "handle domain \"%s\"", requestedDomainName)
			doForward = true
		}
	}
	// If queried domain is not black listed, forward the query to forwarder.
	if doForward {
		// Ask a randomly chosen TCP forwarder to process the query
		myForwarder, err := net.DialTimeout("tcp", daemon.Forwarders[rand.Intn(len(daemon.Forwarders))], IOTimeoutSec*time.Second)
		if err != nil {
			daemon.logger.Warningf("HandleTCPQuery", clientIP, err, "failed to connect to forwarder")
			return
		}
		defer myForwarder.Close()
		// Send original query to forwarder without modification
		myForwarder.SetDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		if _, err = myForwarder.Write(queryLenBuf); err != nil {
			daemon.logger.Warningf("HandleTCPQuery", clientIP, err, "failed to write length to forwarder")
			return
		} else if _, err = myForwarder.Write(queryBuf); err != nil {
			daemon.logger.Warningf("HandleTCPQuery", clientIP, err, "failed to write query to forwarder")
			return
		}
		// Retrieve forwarder's response
		responseLenBuf = make([]byte, 2)
		if _, err = myForwarder.Read(responseLenBuf); err != nil {
			daemon.logger.Warningf("HandleTCPQuery", clientIP, err, "failed to read length from forwarder")
			return
		}
		responseLen = int(responseLenBuf[0])*256 + int(responseLenBuf[1])
		if responseLen > MaxPacketSize || responseLen < 1 {
			daemon.logger.Warningf("HandleTCPQuery", clientIP, nil, "bad response length from forwarder")
			return
		}
		responseBuf = make([]byte, responseLen)
		if _, err = myForwarder.Read(responseBuf); err != nil {
			daemon.logger.Warningf("HandleTCPQuery", clientIP, err, "failed to read response from forwarder")
			return
		}
	}
	// Send response to my client
	if _, err = clientConn.Write(responseLenBuf); err != nil {
		daemon.logger.Warningf("HandleTCPQuery", clientIP, err, "failed to answer length to client")
		return
	} else if _, err = clientConn.Write(responseBuf); err != nil {
		daemon.logger.Warningf("HandleTCPQuery", clientIP, err, "failed to answer to client")
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
	daemon.logger.Printf("StartAndBlockTCP", listenAddr, nil, "going to listen for queries")
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
		go daemon.HandleTCPQuery(clientConn)
	}

}

// Run unit tests on DNS TCP daemon. See TestDNSD_StartAndBlockTCP for daemon setup.
func TestTCPQueries(dnsd *Daemon, t testingstub.T) {
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

	packetBuf := make([]byte, MaxPacketSize)
	success := 0
	// Try to reach rate limit - assume rate limit is 10
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
			if _, err := clientConn.Write(githubComTCPQuery); err != nil {
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
		if _, err := clientConn.Write(githubComTCPQuery); err != nil {
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
