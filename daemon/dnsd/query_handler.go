package dnsd

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
	"golang.org/x/net/dns/dnsmessage"
)

// GetTCPStatsCollector returns stats collector for the TCP server of this daemon.
func (daemon *Daemon) GetTCPStatsCollector() *misc.Stats {
	return misc.DNSDStatsTCP
}

// HandleTCPConnection reads a DNS query from a TCP client and responds to it with the DNS query result.
func (daemon *Daemon) HandleTCPConnection(logger lalog.Logger, ip string, conn *net.TCPConn) {
	misc.TweakTCPConnection(conn, ClientTimeoutSec*time.Second)
	// Read query length
	queryLen := make([]byte, 2)
	_, err := conn.Read(queryLen)
	if err != nil {
		logger.Warning(ip, err, "failed to read query length from client")
		return
	}
	queryLenInteger := int(queryLen[0])*256 + int(queryLen[1])
	// Read query packet
	if queryLenInteger > MaxPacketSize || queryLenInteger < MinNameQuerySize {
		logger.Info(ip, nil, "invalid query length (%d) from client", queryLenInteger)
		return
	}
	queryBody := make([]byte, queryLenInteger)
	_, err = conn.Read(queryBody)
	if err != nil {
		logger.Warning(ip, err, "failed to read query from client")
		return
	}
	// Parse the first (and only) query question.
	parser := new(dnsmessage.Parser)
	header, err := parser.Start(queryBody)
	if err != nil {
		logger.Warning(ip, err, "failed to parse query header")
		return
	}
	question, err := parser.Question()
	if err != nil {
		logger.Warning(ip, err, "failed to parse query question")
		return
	}
	var respBody []byte
	if question.Type == dnsmessage.TypeTXT {
		// The TXT query may be carrying an app command.
		respBody = daemon.handleTextQuery(ip, queryLen, queryBody, header, question)
	} else if question.Type == dnsmessage.TypeNS {
		respBody = daemon.handleNS(ip, queryLen, queryBody, header, question)
	} else if question.Type == dnsmessage.TypeSOA {
		respBody = daemon.handleSOA(ip, queryLen, queryBody, header, question)
	} else {
		// Handle all other query types.
		respBody = daemon.handleNameOrOtherQuery(ip, queryLen, queryBody, header, question)
	}
	// Return early (and close the client connection) in case there is no
	// appropriate response.
	if len(respBody) < 3 {
		return
	}
	// Match the response transaction ID with the request.
	respBody[0] = queryBody[0]
	respBody[1] = queryBody[1]
	// Reset connection IO timeout.
	misc.TweakTCPConnection(conn, ClientTimeoutSec*time.Second)
	respLen := []byte{byte(len(respBody) / 256), byte(len(respBody) % 256)}
	if _, err := conn.Write(respLen); err != nil {
		logger.Warning(ip, err, "failed to answer length to the client")
		return
	} else if _, err := conn.Write(respBody); err != nil {
		logger.Warning(ip, err, "failed to answer to the client")
		return
	}
}

// GetUDPStatsCollector returns stats collector for the UDP server of this daemon.
func (daemon *Daemon) GetUDPStatsCollector() *misc.Stats {
	return misc.DNSDStatsUDP
}

// Read a feature command from each input line, then invoke the requested feature and write the execution result back to client.
func (daemon *Daemon) HandleUDPClient(logger lalog.Logger, ip string, client *net.UDPAddr, packet []byte, srv *net.UDPConn) {
	if len(packet) < MinNameQuerySize {
		logger.Warning(ip, nil, "packet length is too small")
		return
	}
	// Parse the first (and only) query question.
	parser := new(dnsmessage.Parser)
	header, err := parser.Start(packet)
	if err != nil {
		logger.Warning(ip, err, "failed to parse query header")
		return
	}
	question, err := parser.Question()
	if err != nil {
		logger.Warning(ip, err, "failed to parse query question")
		return
	}
	var respBody []byte
	if question.Type == dnsmessage.TypeTXT {
		// The TXT query may be carrying an app command.
		respBody = daemon.handleTextQuery(ip, nil, packet, header, question)
	} else if question.Type == dnsmessage.TypeNS {
		respBody = daemon.handleNS(ip, nil, packet, header, question)
	} else if question.Type == dnsmessage.TypeSOA {
		respBody = daemon.handleSOA(ip, nil, packet, header, question)
	} else {
		// Handle all other query types.
		respBody = daemon.handleNameOrOtherQuery(ip, nil, packet, header, question)
	}
	// Ignore the request if there is no appropriate response
	if len(respBody) < 3 {
		return
	}
	// Match the response transaction ID with the request.
	respBody[0] = packet[0]
	respBody[1] = packet[1]
	// Set deadline for responding to my DNS client because the query reader and
	// response writer do not share the same timeout
	logger.MaybeMinorError(srv.SetWriteDeadline(time.Now().Add(ClientTimeoutSec * time.Second)))
	if _, err := srv.WriteTo(respBody, client); err != nil {
		logger.Warning(ip, err, "failed to answer to client")
		return
	}
}

func (daemon *Daemon) handleTextQuery(clientIP string, queryLen, queryBody []byte, header dnsmessage.Header, question dnsmessage.Question) (respBody []byte) {
	name := question.Name.String()
	if daemon.processQueryTestCaseFunc != nil {
		daemon.processQueryTestCaseFunc(name)
	}
	labels, domainName, numDomainLabels, isRecursive := daemon.queryLabels(name)
	if isRecursive {
		daemon.logger.Info(clientIP, nil, "handling type: %q, name: %q, domain name: %q, number of domain labels: %v, is recursive: %v, recursion desired: %v", question.Type, name, domainName, numDomainLabels, isRecursive, header.RecursionDesired)
		if queryLen == nil {
			return daemon.handleUDPRecursiveQuery(clientIP, queryBody)
		}
		return daemon.handleTCPRecursiveQuery(clientIP, queryLen, queryBody)
	}
	if dtmfDecoded := DecodeDTMFCommandInput(labels); len(dtmfDecoded) > 1 {
		cmdResult := daemon.latestCommands.Execute(context.Background(), daemon.Processor, clientIP, dtmfDecoded)
		daemon.logger.Info(clientIP, nil, "executed a toolbox command")
		// Try to fit the response into a single TXT entry.
		// Keep in mind that by convention DNS uses 512 bytes as the overall
		// message size limit - including both question and response.
		// Leave some buffer room for the DNS headers.
		var err error
		respBody, err = BuildTextResponse(name, header, question, misc.SplitIntoSlice(cmdResult.CombinedOutput, 200, 200))
		if err != nil {
			daemon.logger.Warning(clientIP, err, "failed to build response packet")
		}
	} else {
		daemon.logger.Info(clientIP, nil, "the query has toolbox command prefix but it is exceedingly short")
	}
	return
}

func (daemon *Daemon) handleSOA(clientIP string, queryLen, queryBody []byte, header dnsmessage.Header, question dnsmessage.Question) (respBody []byte) {
	name := question.Name.String()
	_, domainName, numDomainLabels, isRecursive := daemon.queryLabels(name)
	daemon.logger.Info(clientIP, nil, "handling type: %q, name: %q, domain name: %q, number of domain labels: %v, is recursive: %v, recursion desired: %v", question.Type, name, domainName, numDomainLabels, isRecursive, header.RecursionDesired)
	if daemon.processQueryTestCaseFunc != nil {
		daemon.processQueryTestCaseFunc(name)
	}
	if isRecursive {
		if queryLen == nil {
			return daemon.handleUDPRecursiveQuery(clientIP, queryBody)
		}
		return daemon.handleTCPRecursiveQuery(clientIP, queryLen, queryBody)
	}
	respBody, err := BuildSOAResponse(header, question, fmt.Sprintf("ns1.%s.", domainName), domainName)
	if err != nil {
		daemon.logger.Warning(clientIP, err, "failed to build response packet")
	}
	return
}

func (daemon *Daemon) handleNS(clientIP string, queryLen, queryBody []byte, header dnsmessage.Header, question dnsmessage.Question) (respBody []byte) {
	name := question.Name.String()
	_, domainName, numDomainLabels, isRecursive := daemon.queryLabels(name)
	daemon.logger.Info(clientIP, nil, "handling type: %q, name: %q, domain name: %q, number of domain labels: %v, is recursive: %v, recursion desired: %v", question.Type, name, domainName, numDomainLabels, isRecursive, header.RecursionDesired)
	if daemon.processQueryTestCaseFunc != nil {
		daemon.processQueryTestCaseFunc(name)
	}
	if isRecursive {
		if queryLen == nil {
			return daemon.handleUDPRecursiveQuery(clientIP, queryBody)
		}
		return daemon.handleTCPRecursiveQuery(clientIP, queryLen, queryBody)
	}
	respBody, err := BuildNSResponse(header, question, domainName)
	if err != nil {
		daemon.logger.Warning(clientIP, err, "failed to build response packet")
	}
	return
}

func (daemon *Daemon) handleNameOrOtherQuery(clientIP string, queryLen, queryBody []byte, header dnsmessage.Header, question dnsmessage.Question) (respBody []byte) {
	name := question.Name.String()
	_, domainName, numDomainLabels, isRecursive := daemon.queryLabels(name)
	daemon.logger.Info(clientIP, nil, "handling type: %q, name: %q, domain name: %q, number of domain labels: %v, is recursive: %v, recursion desired: %v", question.Type, name, domainName, numDomainLabels, isRecursive, header.RecursionDesired)
	if !isRecursive && len(name) > 0 && name[0] == ProxyPrefix {
		// Non-recursive, send TCP-over-DNS fragment to the proxy.
		if daemon.TCPProxy == nil {
			daemon.logger.Info(clientIP, nil, "received a TCP-over-DNS segment but the server is not configured to handle it.")
			return
		}
		seg := tcpoverdns.SegmentFromDNSName(numDomainLabels, name)
		emptySegment := tcpoverdns.Segment{Flags: tcpoverdns.FlagKeepAlive}
		if seg.Flags.Has(tcpoverdns.FlagMalformed) {
			daemon.logger.Info(clientIP, nil, "received a malformed TCP-over-DNS segment")
			respBody, _ = daemon.TCPOverDNSSegmentResponse(header, question, emptySegment.DNSName("r", domainName))
			return respBody
		}
		cname := string(daemon.responseCache.GetOrSet(name, func() []byte {
			respSegment, hasResp := daemon.TCPProxy.Receive(seg)
			if !hasResp {
				return []byte(emptySegment.DNSName("r", domainName))
			}
			return []byte(respSegment.DNSName("r", domainName))
		}))
		respBody, err := daemon.TCPOverDNSSegmentResponse(header, question, cname)
		if err != nil {
			daemon.logger.Info(clientIP, err, "failed to construct DNS query response for TCP-over-DNS segment")
			return nil
		}
		return respBody
	} else if !isRecursive {
		// Non-recursive, other name queries. There must be a response.
		// Recursive resolvers have a habit of resolving a shorter version (e.g.
		// b.example.com) of the desired name (a.b.example.com), often missing
		// a couple of the leading labels, before resolving the actual name
		// demanded by DNS clients. Without a valid response the recursive
		// resolver will consider the DNS authoritative server unresponsive.
		var err error
		respBody, err = BuildIPv4AddrResponse(header, question, daemon.myPublicIP)
		if err != nil {
			daemon.logger.Info(clientIP, err, "failed to construct DNS query response")
			return
		}
	} else {
		// Recursive queries.
		if daemon.processQueryTestCaseFunc != nil {
			daemon.processQueryTestCaseFunc(name)
		}
		if daemon.IsInBlacklist(name) {
			daemon.logger.Info(clientIP, nil, "handle black-listed name query %q", name)
			respBody, err := BuildBlackHoleAddrResponse(header, question)
			if err != nil {
				daemon.logger.Warning(clientIP, err, "failed to build response packet")
				return nil
			}
			return respBody
		}
		daemon.logger.Info(clientIP, nil, "handle recursive non-name query")
		if queryLen == nil {
			return daemon.handleUDPRecursiveQuery(clientIP, queryBody)
		}
		return daemon.handleTCPRecursiveQuery(clientIP, queryLen, queryBody)
	}
	return
}

/*
handleTCPRecursiveQuery forward the input query to a randomly chosen recursive resolver and retrieves the response.
Be aware that toolbox command processor may invoke this function with an incorrect PIN entry similar to the real PIN,
therefore this function must not log the input packet content in any way.
*/
func (daemon *Daemon) handleTCPRecursiveQuery(clientIP string, queryLen, queryBody []byte) (respBody []byte) {
	respBody = make([]byte, 0)
	if !daemon.isRecursiveQueryAllowed(clientIP) {
		daemon.logger.Warning(clientIP, nil, "client IP is not allowed to query")
		return
	}
	randForwarder := daemon.Forwarders[rand.Intn(len(daemon.Forwarders))]
	// Forward the query to a randomly chosen recursive resolver
	myForwarder, err := net.DialTimeout("tcp", randForwarder, ForwarderTimeoutSec*time.Second)
	if err != nil {
		daemon.logger.Warning(clientIP, err, "failed to connect to forwarder")
		return
	}
	defer func() {
		daemon.logger.MaybeMinorError(myForwarder.Close())
	}()
	// Send original query to the resolver without modification
	daemon.logger.MaybeMinorError(myForwarder.SetDeadline(time.Now().Add(ForwarderTimeoutSec * time.Second)))
	if _, err = myForwarder.Write(queryLen); err != nil {
		daemon.logger.Warning(clientIP, err, "failed to write length to forwarder")
		return
	} else if _, err = myForwarder.Write(queryBody); err != nil {
		daemon.logger.Warning(clientIP, err, "failed to write query to forwarder")
		return
	}
	// Read resolver's response
	respLen := make([]byte, 2)
	if _, err = myForwarder.Read(respLen); err != nil {
		daemon.logger.Warning(clientIP, err, "failed to read length from forwarder")
		return
	}
	respLenInt := int(respLen[0])*256 + int(respLen[1])
	if respLenInt > MaxPacketSize || respLenInt < 1 {
		daemon.logger.Warning(clientIP, nil, "bad response length from forwarder")
		return
	}
	respBody = make([]byte, respLenInt)
	if _, err = myForwarder.Read(respBody); err != nil {
		daemon.logger.Warning(clientIP, err, "failed to read response from forwarder")
		return
	}
	return
}

/*
handleUDPRecursiveQuery forward the input query to a randomly chosen recursive resolver and retrieves the response.
Be aware that toolbox command processor may invoke this function with an incorrect PIN entry similar to the real PIN,
therefore this function must not log the input packet content in any way.
*/
func (daemon *Daemon) handleUDPRecursiveQuery(clientIP string, queryBody []byte) (respBody []byte) {
	respBody = make([]byte, 0)
	if !daemon.isRecursiveQueryAllowed(clientIP) {
		daemon.logger.Info(clientIP, nil, "client IP is not allowed to query")
		return
	}
	// Forward the query to a randomly chosen recursive resolver and return its response
	randForwarder := daemon.Forwarders[rand.Intn(len(daemon.Forwarders))]
	forwarderConn, err := net.DialTimeout("udp", randForwarder, ForwarderTimeoutSec*time.Second)
	if err != nil {
		daemon.logger.Warning(clientIP, err, "failed to dial forwarder's address")
		return
	}
	defer func() {
		daemon.logger.MaybeMinorError(forwarderConn.Close())
	}()
	daemon.logger.MaybeMinorError(forwarderConn.SetDeadline(time.Now().Add(ForwarderTimeoutSec * time.Second)))
	if _, err := forwarderConn.Write(queryBody); err != nil {
		daemon.logger.Warning(clientIP, err, "failed to write to forwarder")
		return
	}
	respBody = make([]byte, MaxPacketSize)
	respLenInt, err := forwarderConn.Read(respBody)
	if err != nil {
		daemon.logger.Warning(clientIP, err, "failed to read from forwarder")
		return
	}
	if respLenInt < 3 {
		daemon.logger.Warning(clientIP, err, "forwarder response is abnormally small")
		return
	}
	respBody = respBody[:respLenInt]
	return
}
