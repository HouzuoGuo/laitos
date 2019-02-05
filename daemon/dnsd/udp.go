package dnsd

import (
	"context"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/httpd/handler"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/HouzuoGuo/laitos/toolbox/filter"
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
	defer func() {
		daemon.logger.MaybeError(udpServer.Close())
	}()
	daemon.udpListener = udpServer
	daemon.logger.Info("StartAndBlockUDP", listenAddr, nil, "going to monitor for queries")
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
			return fmt.Errorf("DNSD.StartAndBlockUDP: failed to accept request - %v", err)
		}
		// Check address against rate limit and allowed IP prefixes
		clientIP := clientAddr.IP.String()
		if !daemon.rateLimit.Add(clientIP, true) {
			continue
		}
		if packetLength < MinNameQuerySize {
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
		respLenInt, respBody = daemon.handleUDPTextQuery(clientIP, queryBody)
	} else {
		// Handle other query types such as name query
		respLenInt, respBody = daemon.handleUDPNameOrOtherQuery(clientIP, queryBody)
	}
	// Ignore the request if there is no appropriate response
	if respBody == nil || len(respBody) < 3 {
		return
	}
	// Send response to the client, match transaction ID of original query.
	respBody[0] = queryBody[0]
	respBody[1] = queryBody[1]
	// Set deadline for responding to my DNS client because the query reader and response writer do not share the same timeout
	daemon.logger.MaybeError(daemon.udpListener.SetWriteDeadline(time.Now().Add(ClientTimeoutSec * time.Second)))
	if _, err := daemon.udpListener.WriteTo(respBody[:respLenInt], client); err != nil {
		daemon.logger.Warning("HandleUDPQuery", clientIP, err, "failed to answer to client")
		return
	}
}

func (daemon *Daemon) handleUDPTextQuery(clientIP string, queryBody []byte) (respLenInt int, respBody []byte) {
	respBody = make([]byte, 0)
	queriedName, commandDTMF := ExtractTextQueryCommandInput(queryBody)
	if daemon.processQueryTestCaseFunc != nil {
		daemon.processQueryTestCaseFunc(queriedName)
	}
	if dtmfDecoded := handler.DTMFDecode(commandDTMF); len(dtmfDecoded) > 1 {
		// If client is repeating the request rapidly, then respond with the previous output.
		var cmdResult *toolbox.Result
		if prevResult := daemon.repeatLastCommandOutput(dtmfDecoded); prevResult != nil {
			cmdResult = prevResult
		} else {
			cmdResult = daemon.Processor.Process(toolbox.Command{
				TimeoutSec: ClientTimeoutSec,
				Content:    dtmfDecoded,
			}, true)
		}
		daemon.latestCommandTimestamp = time.Now().Unix()
		daemon.latestCommandInput = dtmfDecoded
		daemon.latestCommandOutput = cmdResult
		if cmdResult.Error == filter.ErrPINAndShortcutNotFound {
			/*
				Because the prefix may appear in an ordinary text record query that is not a toolbox command, when there is
				a PIN mismatch, forward to recursive resolver as if the query is indeed not a toolbox command.
			*/
			daemon.logger.Info("handleUDPTextQuery", clientIP, nil, "input has command prefix but failed PIN check")
			goto forwardToRecursiveResolver
		} else {
			daemon.logger.Info("handleUDPTextQuery", clientIP, nil, "processed a toolbox command")
			respBody = MakeTextResponse(queryBody, cmdResult.CombinedOutput)
			return len(respBody), respBody
		}
	} else {
		daemon.logger.Info("handleUDPTextQuery", clientIP, nil, "handle query \"%s\"", string(queriedName))
	}
forwardToRecursiveResolver:
	// There's a chance of being a typo in the PIN entry, make sure this function does not log the request input.
	return daemon.handleUDPRecursiveQuery(clientIP, queryBody)
}

func (daemon *Daemon) handleUDPNameOrOtherQuery(clientIP string, queryBody []byte) (respLenInt int, respBody []byte) {
	respBody = make([]byte, 0)
	// Handle other query types such as name query
	domainName := ExtractDomainName(queryBody)
	if domainName == "" {
		daemon.logger.Info("handleUDPNameOrOtherQuery", clientIP, nil, "handle non-name query")
	} else {
		if daemon.processQueryTestCaseFunc != nil {
			daemon.processQueryTestCaseFunc(domainName)
		}
		daemon.logger.Info("handleUDPNameOrOtherQuery", clientIP, nil, "handle query \"%s\"", domainName)
	}
	if daemon.IsInBlacklist(domainName) {
		// Formulate a black-hole response to black-listed domain name
		daemon.logger.Info("handleUDPNameOrOtherQuery", clientIP, nil, "handle black-listed \"%s\"", domainName)
		respBody = GetBlackHoleResponse(queryBody)
		respLenInt = len(respBody)
		return
	}
	return daemon.handleUDPRecursiveQuery(clientIP, queryBody)
}

/*
handleUDPRecursiveQuery forward the input query to a randomly chosen recursive resolver and retrieves the response.
Be aware that toolbox command processor may invoke this function with an incorrect PIN entry similar to the real PIN,
therefore this function must not log the input packet content in any way.
*/
func (daemon *Daemon) handleUDPRecursiveQuery(clientIP string, queryBody []byte) (respLenInt int, respBody []byte) {
	respBody = make([]byte, 0)
	if !daemon.checkAllowClientIP(clientIP) {
		daemon.logger.Warning("handleUDPRecursiveQuery", clientIP, nil, "client IP is not allowed to query")
		return
	}
	// Forward the query to a randomly chosen recursive resolver and return its response
	randForwarder := daemon.Forwarders[rand.Intn(len(daemon.Forwarders))]
	forwarderConn, err := net.DialTimeout("udp", randForwarder, ForwarderTimeoutSec*time.Second)
	if err != nil {
		daemon.logger.Warning("handleUDPRecursiveQuery", clientIP, err, "failed to dial forwarder's address")
		return
	}
	daemon.logger.MaybeError(forwarderConn.SetDeadline(time.Now().Add(ForwarderTimeoutSec * time.Second)))
	if _, err := forwarderConn.Write(queryBody); err != nil {
		daemon.logger.Warning("handleUDPRecursiveQuery", clientIP, err, "failed to write to forwarder")
		return
	}
	respBody = make([]byte, MaxPacketSize)
	respLenInt, err = forwarderConn.Read(respBody)
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

	// Use go DNS client to verify that the server returns satisfactory response
	resolver := &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (conn net.Conn, e error) {
			return net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", dnsd.UDPPort))
		},
	}
	testResolveNameAndBlackList(t, dnsd, resolver, true)
	// Try to flood the server and reach rate limit
	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+strconv.Itoa(dnsd.UDPPort))
	if err != nil {
		t.Fatal(err)
	}
	packetBuf := make([]byte, MaxPacketSize)
	var success int
	for i := 0; i < 40; i++ {
		go func() {
			clientConn, err := net.DialUDP("udp", nil, serverAddr)
			if err != nil {
				t.Fatal(err)
			}
			if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				lalog.DefaultLogger.MaybeError(clientConn.Close())
				t.Fatal(err)
			}
			if _, err := clientConn.Write(githubComUDPQuery); err != nil {
				lalog.DefaultLogger.MaybeError(clientConn.Close())
				t.Fatal(err)
			}
			length, err := clientConn.Read(packetBuf)
			lalog.DefaultLogger.MaybeError(clientConn.Close())
			if err == nil && length > 50 {
				success++
			}
		}()
	}
	// Wait for rate limit to reset and verify that regular name resolution resumes
	time.Sleep(RateLimitIntervalSec * time.Second)
	testResolveNameAndBlackList(t, dnsd, resolver, true)
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
