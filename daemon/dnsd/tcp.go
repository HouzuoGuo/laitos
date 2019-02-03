package dnsd

import (
	"context"
	"fmt"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/HouzuoGuo/laitos/toolbox/filter"
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
	defer func() {
		daemon.logger.MaybeError(listener.Close())
	}()
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
			daemon.logger.MaybeError(clientConn.Close())
			continue
		}
		go daemon.handleTCPQuery(clientConn)
	}
}

func (daemon *Daemon) handleTCPQuery(clientConn net.Conn) {
	// Put query duration (including IO time) into statistics
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		daemon.logger.MaybeError(clientConn.Close())
		common.DNSDStatsTCP.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()
	clientIP := clientConn.RemoteAddr().(*net.TCPAddr).IP.String()
	// Read query length
	daemon.logger.MaybeError(clientConn.SetDeadline(time.Now().Add(ClientTimeoutSec * time.Second)))
	queryLen := make([]byte, 2)
	_, err := clientConn.Read(queryLen)
	if err != nil {
		daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to read query length from client")
		return
	}
	queryLenInteger := int(queryLen[0])*256 + int(queryLen[1])
	// Read query packet
	if queryLenInteger > MaxPacketSize || queryLenInteger < MinNameQuerySize {
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
	var respBody, respLen []byte
	if isTextQuery(queryBody) {
		// Handle toolbox command that arrives as a text query
		respLen, respBody = daemon.handleTCPTextQuery(clientIP, queryLen, queryBody)
	} else {
		// Handle other query types such as name query
		respLen, respBody = daemon.handleTCPNameOrOtherQuery(clientIP, queryLen, queryBody)
	}
	// Close client connection in case there is no appropriate response
	if respBody == nil || len(respBody) < 2 {
		return
	}
	// Send response to the client, match transaction ID of original query, the deadline is shared with the read deadline above.
	respBody[0] = queryBody[0]
	respBody[1] = queryBody[1]
	if _, err := clientConn.Write(respLen); err != nil {
		daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to answer length to client")
		return
	} else if _, err := clientConn.Write(respBody); err != nil {
		daemon.logger.Warning("handleTCPQuery", clientIP, err, "failed to answer to client")
		return
	}
}

func (daemon *Daemon) handleTCPTextQuery(clientIP string, queryLen, queryBody []byte) (respLen, respBody []byte) {
	respBody = make([]byte, 0)
	queryInput := ExtractTextQueryInput(queryBody)
	queriedNameForLogging := []byte(queryInput)
	recoverFullStopSymbols(queriedNameForLogging)
	if daemon.processQueryTestCaseFunc != nil {
		daemon.processQueryTestCaseFunc(string(queriedNameForLogging))
	}
	if strings.HasPrefix(queryInput, ToolboxCommandPrefix) {
		inputWithoutPrefix := strings.TrimPrefix(queryInput, ToolboxCommandPrefix)
		result := daemon.Processor.Process(toolbox.Command{
			TimeoutSec: ClientTimeoutSec,
			Content:    inputWithoutPrefix,
		}, true)
		if result.Error == filter.ErrPINAndShortcutNotFound {
			/*
				Because the prefix may appear in an ordinary text record query that is not a toolbox command, when there is
				a PIN mismatch, forward to recursive resolver as if the query is indeed not a toolbox command.
			*/
			daemon.logger.Info("handleUDPTextQuery", clientIP, nil, "input has command prefix but failed PIN check, forward to recursive resolver.")
			goto forwardToRecursiveResolver
		} else {
			daemon.logger.Info("handleUDPTextQuery", clientIP, nil, "processed a toolbox command")
			respBody = MakeTextResponse(queryBody, result.CombinedOutput)
			respLenInt := len(respBody)
			respLen = []byte{byte(respLenInt / 256), byte(respLenInt % 256)}
			return
		}
	}
forwardToRecursiveResolver:
	// There's a chance of being a typo in the PIN entry, make sure this function does not log the request input.
	return daemon.handleTCPRecursiveQuery(clientIP, queryLen, queryBody)
}

func (daemon *Daemon) handleTCPNameOrOtherQuery(clientIP string, queryLen, queryBody []byte) (respLen, respBody []byte) {
	respLen = make([]byte, 0)
	respBody = make([]byte, 0)
	if !daemon.checkAllowClientIP(clientIP) {
		daemon.logger.Warning("handleTCPNameOrOtherQuery", clientIP, nil, "client IP is not allowed to query")
		return
	}
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
		respLenInt := len(respBody)
		respLen = []byte{byte(respLenInt / 256), byte(respLenInt % 256)}
	} else {
		respLen, respBody = daemon.handleTCPRecursiveQuery(clientIP, queryLen, queryBody)
	}
	return
}

/*
handleTCPRecursiveQuery forward the input query to a randomly chosen recursive resolver and retrieves the response.
Be aware that toolbox command processor may invoke this function with an incorrect PIN entry similar to the real PIN,
therefore this function must not log the input packet content in any way.
*/
func (daemon *Daemon) handleTCPRecursiveQuery(clientIP string, queryLen, queryBody []byte) (respLen, respBody []byte) {
	respLen = make([]byte, 0)
	respBody = make([]byte, 0)
	if !daemon.checkAllowClientIP(clientIP) {
		daemon.logger.Warning("handleTCPRecursiveQuery", clientIP, nil, "client IP is not allowed to query")
		return
	}
	randForwarder := daemon.Forwarders[rand.Intn(len(daemon.Forwarders))]
	// Forward the query to a randomly chosen recursive resolver
	myForwarder, err := net.DialTimeout("tcp", randForwarder, ForwarderTimeoutSec*time.Second)
	if err != nil {
		daemon.logger.Warning("handleTCPRecursiveQuery", clientIP, err, "failed to connect to forwarder")
		return
	}
	defer func() {
		daemon.logger.MaybeError(myForwarder.Close())
	}()
	// Send original query to the resolver without modification
	daemon.logger.MaybeError(myForwarder.SetDeadline(time.Now().Add(ForwarderTimeoutSec * time.Second)))
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
	respLenInt := int(respLen[0])*256 + int(respLen[1])
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
	testResolveNameAndBlackList(t, dnsd, resolver, false)
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
				lalog.DefaultLogger.MaybeError(clientConn.Close())
				t.Fatal(err)
			}
			if _, err := clientConn.Write(githubComTCPQuery); err != nil {
				lalog.DefaultLogger.MaybeError(clientConn.Close())
				t.Fatal(err)
			}
			resp, err := ioutil.ReadAll(clientConn)
			lalog.DefaultLogger.MaybeError(clientConn.Close())
			if err == nil && len(resp) > 50 {
				success++
			}
		}()
	}
	// Wait for rate limit to reset and verify that regular name resolution resumes
	time.Sleep(RateLimitIntervalSec * time.Second)
	testResolveNameAndBlackList(t, dnsd, resolver, false)
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
