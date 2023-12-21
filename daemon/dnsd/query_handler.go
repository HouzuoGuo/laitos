package dnsd

import (
	"context"
	"errors"
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
func (daemon *Daemon) HandleTCPConnection(logger *lalog.Logger, ip string, conn *net.TCPConn) {
	misc.TweakTCPConnection(conn, ClientTimeoutSec*time.Second)
	// Read query length
	queryLen := make([]byte, 2)
	n, err := conn.Read(queryLen)
	if err != nil || n != 2 {
		logger.Info(ip, err, "failed to read query length from client, read %d bytes.", n)
		return
	}
	queryLenInteger := int(queryLen[0])*256 + int(queryLen[1])

	// Read query packet
	if queryLenInteger > MaxPacketSize || queryLenInteger < MinNameQuerySize {
		logger.Info(ip, nil, "invalid query length (%d) from client", queryLenInteger)
		return
	}
	queryBody := make([]byte, queryLenInteger)
	n, err = conn.Read(queryBody)
	if err != nil || n != queryLenInteger {
		logger.Warning(ip, err, "failed to read query from client (read %d bytes)", n)
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
	} else if question.Type == dnsmessage.TypeMX {
		respBody = daemon.handleMX(ip, queryLen, queryBody, header, question)
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
func (daemon *Daemon) HandleUDPClient(logger *lalog.Logger, ip string, client *net.UDPAddr, packet []byte, srv *net.UDPConn) {
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
	} else if question.Type == dnsmessage.TypeMX {
		respBody = daemon.handleMX(ip, nil, packet, header, question)
	} else {
		// Handle all other query types.
		respBody = daemon.handleNameOrOtherQuery(ip, nil, packet, header, question)
	}
	// Ignore the request if there is no appropriate response
	if len(respBody) < MinNameQuerySize {
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

func (daemon *Daemon) handleTCPOverDNSQuery(header dnsmessage.Header, question dnsmessage.Question, clientIP string) ([]byte, error) {
	name := question.Name.String()
	_, domainName, numDomainLabels, _, _ := daemon.queryLabels(name)
	if daemon.TCPProxy == nil || daemon.TCPProxy.RequestOTPSecret == "" {
		daemon.logger.Info(clientIP, nil, "received a TCP-over-DNS segment but the server is not configured to handle it.")
		return nil, errors.New("missing tcp-over-dns server config")
	}
	requestSeg := tcpoverdns.SegmentFromDNSName(numDomainLabels, name)
	emptyResposneSeg := tcpoverdns.Segment{Flags: tcpoverdns.FlagKeepAlive}
	if requestSeg.Flags.Has(tcpoverdns.FlagMalformed) {
		daemon.logger.Info(clientIP, nil, "received a malformed TCP-over-DNS segment")
		return BuildTCPOverDNSSegmentResponse(header, question, domainName, emptyResposneSeg)
	}
	cachedResponseSeg := daemon.responseCache.GetOrSet(name, func() tcpoverdns.Segment {
		respSegment, hasResp := daemon.TCPProxy.Receive(requestSeg)
		if !hasResp {
			return emptyResposneSeg
		}
		return respSegment
	})
	respBody, err := BuildTCPOverDNSSegmentResponse(header, question, domainName, cachedResponseSeg)
	if err != nil {
		daemon.logger.Info(clientIP, err, "failed to construct DNS query response for TCP-over-DNS segment")
		return nil, err
	}
	return respBody, nil
}

func (daemon *Daemon) handleTextQuery(clientIP string, queryLen, queryBody []byte, header dnsmessage.Header, question dnsmessage.Question) (respBody []byte) {
	name := question.Name.String()
	if daemon.processQueryTestCaseFunc != nil {
		daemon.processQueryTestCaseFunc(name)
	}
	labels, domainName, numDomainLabels, isRecursive, _ := daemon.queryLabels(name)
	if isRecursive {
		if !daemon.queryRateLimit.Add(clientIP, true) {
			return
		}
		daemon.logger.Info(clientIP, nil, "handling type: %q, name: %q, domain name: %q, number of domain labels: %v, is recursive: %v, recursion desired: %v", question.Type, name, domainName, numDomainLabels, isRecursive, header.RecursionDesired)
		if queryLen == nil {
			return daemon.handleUDPRecursiveQuery(clientIP, queryBody)
		}
		return daemon.handleTCPRecursiveQuery(clientIP, queryLen, queryBody)
	}
	// The query is directed at the laitos DNS server itself.
	var err error
	if name[0] == ToolboxCommandPrefix {
		if !daemon.queryRateLimit.Add(clientIP, true) {
			return
		}
		// The query could be an app command.
		if dtmfDecoded := DecodeDTMFCommandInput(labels); len(dtmfDecoded) > 3 {
			cmdResult := daemon.latestCommands.Execute(context.Background(), daemon.Processor, clientIP, dtmfDecoded)
			daemon.logger.Info(clientIP, nil, "executed a toolbox command")
			// Try to fit the response into a single TXT entry.
			// Keep in mind that by convention DNS uses 512 bytes as the overall
			// message size limit - including both question and response.
			// Leave some buffer room for the DNS headers.
			respBody, err = BuildTextResponse(name, header, question, misc.SplitIntoSlice(cmdResult.CombinedOutput, 200, 200))
			if err != nil {
				daemon.logger.Warning(clientIP, err, "failed to build response packet")
			}
		} else {
			daemon.logger.Info(clientIP, nil, "the query has toolbox command prefix but it is exceedingly short")
		}
	} else if name[0] == ProxyPrefix {
		// PerIPLimit rather than PerIPQueryLimit applies.
		respBody, _ = daemon.handleTCPOverDNSQuery(header, question, clientIP)
	} else {
		// Or just a regular dig.
		if !daemon.queryRateLimit.Add(clientIP, true) {
			return
		}
		daemon.logger.Info(clientIP, nil, "handling type: %q, name: %q, domain name: %q, number of domain labels: %v, is recursive: %v, recursion desired: %v", question.Type, name, domainName, numDomainLabels, isRecursive, header.RecursionDesired)
		respBody, err = BuildTextResponse(name, header, question, []string{fmt.Sprintf(`v=spf1 mx a mx:%s ?all`, domainName)})
		if err != nil {
			daemon.logger.Warning(clientIP, err, "failed to build response packet")
		}
	}
	return
}

func (daemon *Daemon) handleSOA(clientIP string, queryLen, queryBody []byte, header dnsmessage.Header, question dnsmessage.Question) (respBody []byte) {
	if !daemon.queryRateLimit.Add(clientIP, true) {
		return
	}
	name := question.Name.String()
	_, domainName, numDomainLabels, isRecursive, _ := daemon.queryLabels(name)
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
	respBody, err := BuildSOAResponse(header, question, fmt.Sprintf("ns1.%s.", domainName), "webmaster@"+domainName)
	if err != nil {
		daemon.logger.Warning(clientIP, err, "failed to build response packet")
	}
	return
}

func (daemon *Daemon) handleMX(clientIP string, queryLen, queryBody []byte, header dnsmessage.Header, question dnsmessage.Question) (respBody []byte) {
	if !daemon.queryRateLimit.Add(clientIP, true) {
		return
	}
	name := question.Name.String()
	_, domainName, numDomainLabels, isRecursive, customRec := daemon.queryLabels(name)
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
	var err error
	if customRec == nil {
		// The DNS daemon will happily resolve all non-recursive address queries to
		// its own public IP address.
		mx := []MXRecord{
			{
				Priority: 10,
				Name:     lintDNSName(fmt.Sprintf("mx.%s.", domainName)),
			},
		}
		respBody, err = BuildMXResponse(header, question, mx)
	} else {
		respBody, err = BuildMXResponse(header, question, customRec.MX)
	}
	if err != nil {
		daemon.logger.Warning(clientIP, err, "failed to build response packet")
	}
	return
}

func (daemon *Daemon) handleNS(clientIP string, queryLen, queryBody []byte, header dnsmessage.Header, question dnsmessage.Question) (respBody []byte) {
	if !daemon.queryRateLimit.Add(clientIP, true) {
		return
	}
	name := question.Name.String()
	_, domainName, numDomainLabels, isRecursive, customRec := daemon.queryLabels(name)
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
	var err error
	if customRec == nil {
		respBody, err = BuildNSResponse(header, question, domainName, customRec.NS, net.IPv4zero)
	} else {
		// The DNS daemon will happily resolve all non-recursive address queries
		// to its own public IP address.
		ns := NSRecord{
			Names: []string{
				fmt.Sprintf("ns1.%s.", domainName),
				fmt.Sprintf("ns2.%s.", domainName),
				fmt.Sprintf("ns3.%s.", domainName),
			},
		}
		respBody, err = BuildNSResponse(header, question, domainName, ns, daemon.myPublicIP)
	}
	if err != nil {
		daemon.logger.Warning(clientIP, err, "failed to build response packet")
	}
	return
}

func (daemon *Daemon) handleNameOrOtherQuery(clientIP string, queryLen, queryBody []byte, header dnsmessage.Header, question dnsmessage.Question) (respBody []byte) {
	name := question.Name.String()
	_, domainName, numDomainLabels, isRecursive, customRec := daemon.queryLabels(name)
	daemon.logger.Info(clientIP, nil, "handling type: %q, name: %q, domain name: %q, number of domain labels: %v, is recursive: %v, recursion desired: %v", question.Type, name, domainName, numDomainLabels, isRecursive, header.RecursionDesired)
	var err error
	if isRecursive {
		// Act as a stub resolver and forward the request.
		if !daemon.queryRateLimit.Add(clientIP, true) {
			return
		}
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
	if len(name) > 0 && name[0] == ProxyPrefix {
		// Handle TCP-over-DNS query.
		// Caller's PerIPLimit check applies, the PerIPQueryLimit does not apply.
		respBody, _ = daemon.handleTCPOverDNSQuery(header, question, clientIP)
		return respBody
	} else if customRec != nil {
		if !daemon.queryRateLimit.Add(clientIP, true) {
			return
		}
		if question.Type == dnsmessage.TypeA {
			respBody, err = BuildIPv4AddrResponse(header, question, customRec.A)
		} else {
			respBody, err = BuildIPv6AddrResponse(header, question, customRec.AAAA)
		}
		if err != nil {
			daemon.logger.Info(clientIP, err, "failed to construct DNS query response")
			return
		}
	} else {
		// Answer to all other name queries with server's own IP.
		// First and foremost this helps responding to ns#.* and mx.*
		// for the server's own domain names. In addition, recursive resolvers
		// have an unusual habit of resolving a shorter version
		// (e.g. b.example.com) of the desired name (a.b.example.com),
		// often missing a couple of the leading labels, before resolving the
		// actual name demanded by DNS clients. Without a valid response the
		// recursive resolver will consider the DNS authoritative server
		// unresponsive.
		if !daemon.queryRateLimit.Add(clientIP, true) {
			return
		}
		respBody, err = BuildIPv4AddrResponse(header, question, V4AddressRecord{
			AddressRecord: AddressRecord{ipAddresses: []net.IP{daemon.myPublicIP}},
		})
		if err != nil {
			daemon.logger.Info(clientIP, err, "failed to construct DNS query response")
			return
		}
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
		daemon.logger.Info(clientIP, nil, "client IP is not allowed to query")
		return
	}
	var forwarder net.Conn
	var err error
	if daemon.DNSRelay == nil {
		randForwarder := daemon.Forwarders[rand.Intn(len(daemon.Forwarders))]
		// Forward the query to a randomly chosen recursive resolver
		forwarder, err = net.DialTimeout("tcp", randForwarder, ForwarderTimeoutSec*time.Second)
		if err != nil {
			daemon.logger.Warning(clientIP, err, "failed to connect to forwarder")
			return
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		tc := daemon.DNSRelay.TransmissionControl(ctx)
		defer cancel()
		if tc == nil {
			daemon.logger.Warning(clientIP, err, "relay's transmission control failed to reach established state")
			return nil
		}
		forwarder = tc
	}
	defer func() {
		daemon.logger.MaybeMinorError(forwarder.Close())
	}()
	// Send original query to the resolver without modification.
	daemon.logger.MaybeMinorError(forwarder.SetDeadline(time.Now().Add(ForwarderTimeoutSec * time.Second)))
	if daemon.DNSRelay != nil {
		daemon.DNSRelay.TransactionMutex.Lock()
		defer func() {
			daemon.DNSRelay.TransactionMutex.Unlock()
		}()
	}
	if _, err = forwarder.Write(queryLen); err != nil {
		daemon.logger.Warning(clientIP, err, "failed to write length to forwarder")
		return
	} else if _, err = forwarder.Write(queryBody); err != nil {
		daemon.logger.Warning(clientIP, err, "failed to write query to forwarder")
		return
	}
	// Read resolver's response.
	respLen := make([]byte, 2)
	if _, err = forwarder.Read(respLen); err != nil {
		daemon.logger.Warning(clientIP, err, "failed to read length from forwarder")
		return
	}
	respLenInt := int(respLen[0])*256 + int(respLen[1])
	if respLenInt > MaxPacketSize || respLenInt < 1 {
		daemon.logger.Warning(clientIP, nil, "bad response length from forwarder")
		return
	}
	respBody = make([]byte, respLenInt)
	if _, err = forwarder.Read(respBody); err != nil {
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
	if daemon.DNSRelay == nil {
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
	// Forward using the TCP-over-DNS relay.
	// Send the query length and query body.
	if daemon.DNSRelay != nil {
		daemon.DNSRelay.TransactionMutex.Lock()
		defer func() {
			daemon.DNSRelay.TransactionMutex.Unlock()
		}()
	}
	queryLen := []byte{byte(len(queryBody) / 256), byte(len(queryBody) % 256)}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tc := daemon.DNSRelay.TransmissionControl(ctx)
	if tc == nil {
		daemon.logger.Warning(clientIP, nil, "relay's transmission control failed to reach established state")
		return nil
	}
	defer func() {
		daemon.logger.MaybeMinorError(tc.Close())
	}()
	if _, err := tc.Write(queryLen); err != nil {
		daemon.logger.Warning(clientIP, err, "failed to write query length via DNS relay")
		return
	}
	if _, err := tc.Write(queryBody); err != nil {
		daemon.logger.Warning(clientIP, err, "failed to write query body via DNS relay")
		return
	}
	respLen := make([]byte, 2)
	_, err := tc.Read(respLen)
	if err != nil {
		daemon.logger.Warning(clientIP, err, "failed to read response length via DNS relay")
		tc.Close()
		return
	}
	// Read the response length and response body.
	respLenInt := int(respLen[0])*256 + int(respLen[1])
	if respLenInt > MaxPacketSize || respLenInt < MinNameQuerySize {
		daemon.logger.Info(clientIP, nil, "received invalid response length (%d) via DNS relay", respLenInt)
		tc.Close()
		return
	}
	respBody = make([]byte, respLenInt)
	_, err = tc.Read(respBody)
	if err != nil {
		daemon.logger.Warning(clientIP, err, "failed to read response via DNS relay")
		tc.Close()
		return
	}
	return
}
