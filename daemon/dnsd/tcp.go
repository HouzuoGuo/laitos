package dnsd

import (
	"context"
	"fmt"
	"io/ioutil"
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
StartAndBlockTCP starts DNS daemon to listen on TCP port only, until daemon is told to stop.
Daemon must have already been initialised prior to this call.
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

func (daemon *Daemon) handleTCPQuery(clientConn net.Conn) {
	// Put query duration (including IO time) into statistics
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		common.DNSDStatsTCP.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()
	defer clientConn.Close()
	clientIP := clientConn.RemoteAddr().(*net.TCPAddr).IP.String()
	// Read query length
	clientConn.SetDeadline(time.Now().Add(ClientTimeoutSec * time.Second))
	queryLen := make([]byte, 2)
	_, err := clientConn.Read(queryLen)
	if err != nil {
		daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to read query length from client")
		return
	}
	queryLenInteger := int(queryLen[0])*256 + int(queryLen[1])
	// Read query packet
	if queryLenInteger > MaxPacketSize || queryLenInteger < 3 {
		daemon.logger.Warning("handleTCPQuery", clientIP, nil, "invalid query length from client")
		return
	}
	queryBody := make([]byte, queryLenInteger)
	_, err = clientConn.Read(queryBody)
	if err != nil {
		daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to read query from client")
		return
	}
	// Formulate a response
	var respLen []byte
	var respBody []byte
	if isTextQuery(queryBody) {
		// Handle toolbox command that arrives as a text query
		_, respLen, respBody = daemon.handleTCPTextQuery(clientIP, queryLen, queryBody)
	} else {
		// Handle other query types such as name query
		_, respLen, respBody = daemon.handleTCPNameOrOtherQuery(clientIP, queryLen, queryBody)
	}
	// Send response to the client, the deadline is shared with the read deadline above.
	if _, err := clientConn.Write(respLen); err != nil {
		daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to answer length to client")
		return
	} else if _, err := clientConn.Write(respBody); err != nil {
		daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to answer to client")
		return
	}
}

func (daemon *Daemon) handleTCPTextQuery(clientIP string, queryLen, queryBody []byte) (respLenInt int, respLen, respBody []byte) {
	// TODO: implement this
	respLen = make([]byte, 0)
	respBody = make([]byte, 0)
	return daemon.handleTCPRecursiveQuery(clientIP, queryLen, queryBody)
}

func (daemon *Daemon) handleTCPNameOrOtherQuery(clientIP string, queryLen, queryBody []byte) (respLenInt int, respLen, respBody []byte) {
	respLen = make([]byte, 0)
	respBody = make([]byte, 0)
	domainName := ExtractDomainName(queryBody)
	if domainName == "" {
		daemon.logger.Info("handleTCPNameOrOtherQuery", clientIP, nil, "handle non-name query")
	} else {
		if daemon.processQueryTestCaseFunc != nil {
			daemon.processQueryTestCaseFunc(domainName)
		}
		daemon.logger.Info("handleTCPNameOrOtherQuery", clientIP, nil, "handle query \"%s\"", domainName)
	}
	if daemon.IsInBlacklist(domainName) {
		// Black hole response returns a
		daemon.logger.Info("handleTCPNameOrOtherQuery", clientIP, nil, "handle black-listed \"%s\"", domainName)
		respBody = GetBlackHoleResponse(queryBody)
		respLenInt = len(respBody)
		respLen = make([]byte, 2)
		respLen[0] = byte(respLenInt / 256)
		respLen[1] = byte(respLenInt % 256)
	} else {
		respLenInt, respLen, respBody = daemon.handleTCPRecursiveQuery(clientIP, queryLen, queryBody)
	}
	return
}

func (daemon *Daemon) handleTCPRecursiveQuery(clientIP string, queryLen, queryBody []byte) (respLenInt int, respLen, respBody []byte) {
	respLen = make([]byte, 0)
	respBody = make([]byte, 0)
	randForwarder := daemon.Forwarders[rand.Intn(len(daemon.Forwarders))]
	// Forward the query to a randomly chosen recursive resolver
	myForwarder, err := net.DialTimeout("tcp", randForwarder, ForwarderTimeoutSec*time.Second)
	if err != nil {
		daemon.logger.Warning("handleTCPRecursiveQuery", clientIP, err, "failed to connect to forwarder")
		return
	}
	defer myForwarder.Close()
	// Send original query to the resolver without modification
	myForwarder.SetDeadline(time.Now().Add(ForwarderTimeoutSec * time.Second))
	if _, err = myForwarder.Write(queryLen); err != nil {
		daemon.logger.Warning("handleTCPRecursiveQuery", clientIP, err, "failed to write length to forwarder")
		return
	} else if _, err = myForwarder.Write(queryBody); err != nil {
		daemon.logger.Warning("handleTCPRecursiveQuery", clientIP, err, "failed to write query to forwarder")
		return
	}
	// Read resolver's response
	respLen = make([]byte, 2)
	if _, err = myForwarder.Read(respLen); err != nil {
		daemon.logger.Warning("handleTCPRecursiveQuery", clientIP, err, "failed to read length from forwarder")
		return
	}
	respLenInt = int(respLen[0])*256 + int(respLen[1])
	if respLenInt > MaxPacketSize || respLenInt < 1 {
		daemon.logger.Warning("handleTCPRecursiveQuery", clientIP, nil, "bad response length from forwarder")
		return
	}
	respBody = make([]byte, respLenInt)
	if _, err = myForwarder.Read(respBody); err != nil {
		daemon.logger.Warning("handleTCPRecursiveQuery", clientIP, err, "failed to read response from forwarder")
		return
	}
	return
}

// Run unit tests on DNS TCP daemon. See TestDNSD_StartAndBlockTCP for daemon setup.
func TestTCPQueries(dnsd *Daemon, t testingstub.T) {
	if misc.HostIsWindows() {
		// FIXME: fix this test case for Windows
		t.Log("FIXME: enable TestTCPQueries for Windows")
		return
	}
	// Prevent daemon from listening to UDP queries in this TCP test case
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
	// Use go DNS client to verify that the server returns satisfactory response
	resolver := &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (conn net.Conn, e error) {
			return net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", dnsd.TCPPort))
		},
	}
	testResolveNameAndBlackList(t, dnsd, resolver)
	// Try to flood the server and reach rate limit
	success := 0
	dnsd.blackList = map[string]struct{}{}
	for i := 0; i < 40; i++ {
		go func() {
			clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(dnsd.TCPPort))
			if err != nil {
				t.Fatal(err)
			}
			if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				clientConn.Close()
				t.Fatal(err)
			}
			if _, err := clientConn.Write(GithubComTCPQuery); err != nil {
				clientConn.Close()
				t.Fatal(err)
			}
			resp, err := ioutil.ReadAll(clientConn)
			clientConn.Close()
			if err == nil && len(resp) > 50 {
				success++
			}
		}()
	}
	// Wait for rate limit to reset and verify that regular name resolution resumes
	time.Sleep(RateLimitIntervalSec * time.Second)
	testResolveNameAndBlackList(t, dnsd, resolver)
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
