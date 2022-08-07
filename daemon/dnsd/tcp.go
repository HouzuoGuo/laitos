package dnsd

import (
	"context"
	"math/rand"
	"net"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
	"golang.org/x/net/dns/dnsmessage"

	"github.com/HouzuoGuo/laitos/misc"
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
		logger.Warning("HandleTCPConnection", ip, err, "failed to read query length from client")
		return
	}
	queryLenInteger := int(queryLen[0])*256 + int(queryLen[1])
	// Read query packet
	if queryLenInteger > MaxPacketSize || queryLenInteger < MinNameQuerySize {
		logger.Info("HandleTCPConnection", ip, nil, "invalid query length from client")
		return
	}
	queryBody := make([]byte, queryLenInteger)
	_, err = conn.Read(queryBody)
	if err != nil {
		logger.Warning("HandleTCPConnection", ip, err, "failed to read query from client")
		return
	}
	// Parse the first (and only) query question.
	parser := new(dnsmessage.Parser)
	header, err := parser.Start(queryBody)
	if err != nil {
		logger.Warning("HandleTCPConnection", ip, err, "failed to parse query header")
		return
	}
	question, err := parser.Question()
	if err != nil {
		logger.Warning("HandleTCPConnection", ip, err, "failed to parse query question")
		return
	}
	var respBody []byte
	if question.Type == dnsmessage.TypeTXT {
		// The TXT query may be carrying an app command.
		respBody = daemon.handleTCPTextQuery(ip, queryLen, queryBody, header, question)
	} else {
		// Handle all other query types.
		respBody = daemon.handleTCPNameOrOtherQuery(ip, queryLen, queryBody, header, question)
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
		logger.Warning("HandleTCPConnection", ip, err, "failed to answer length to the client")
		return
	} else if _, err := conn.Write(respBody); err != nil {
		logger.Warning("HandleTCPConnection", ip, err, "failed to answer to the client")
		return
	}
}

func (daemon *Daemon) handleTCPTextQuery(clientIP string, queryLen, queryBody []byte, header dnsmessage.Header, question dnsmessage.Question) (respBody []byte) {
	name := question.Name.String()
	if daemon.processQueryTestCaseFunc != nil {
		daemon.processQueryTestCaseFunc(name)
	}
	labels, _, isRecursive := daemon.queryLabels(name)
	if isRecursive {
		return daemon.handleTCPRecursiveQuery(clientIP, queryLen, queryBody)
	}
	if dtmfDecoded := DecodeDTMFCommandInput(labels); len(dtmfDecoded) > 1 {
		cmdResult := daemon.latestCommands.Execute(context.Background(), daemon.Processor, clientIP, dtmfDecoded)
		daemon.logger.Info("handleTCPTextQuery", clientIP, nil, "executed a toolbox command")
		// Try to fit the response into a single TXT entry.
		// Keep in mind that by convention DNS uses 512 bytes as the overall
		// message size limit - including both question and response.
		// Leave some buffer room for the DNS headers.
		var err error
		respBody, err = BuildTextResponse(name, header, question, misc.SplitIntoSlice(cmdResult.CombinedOutput, 200, 200))
		if err != nil {
			daemon.logger.Warning("handleTCPTextQuery", clientIP, err, "failed to build response packet")
		}
	} else {
		daemon.logger.Info("handleTCPTextQuery", clientIP, nil, "the query has toolbox command prefix but it is exceedingly short")
	}
	return
}

func (daemon *Daemon) handleTCPNameOrOtherQuery(clientIP string, queryLen, queryBody []byte, header dnsmessage.Header, question dnsmessage.Question) (respBody []byte) {
	name := question.Name.String()
	_, numDomainLabels, isRecursive := daemon.queryLabels(name)
	if isRecursive {
		if daemon.processQueryTestCaseFunc != nil {
			daemon.processQueryTestCaseFunc(name)
		}
		daemon.logger.Info("handleTCPNameOrOtherQuery", clientIP, nil, "handle name query %q", name)
		if daemon.IsInBlacklist(name) {
			daemon.logger.Info("handleTCPNameOrOtherQuery", clientIP, nil, "handle black-listed name query %q", name)
			respBody, err := BuildBlackHoleAddrResponse(header, question)
			if err != nil {
				daemon.logger.Warning("handleTCPNameOrOtherQuery", clientIP, err, "failed to build response packet")
				return nil
			}
			return respBody
		}
		daemon.logger.Info("handleTCPNameOrOtherQuery", clientIP, nil, "handle non-name query")
		return daemon.handleTCPRecursiveQuery(clientIP, queryLen, queryBody)
	}
	if len(name) > 0 && name[0] == ProxyPrefix {
		// Send TCP-over-DNSOverTCP fragment to the proxy.
		seg := tcpoverdns.SegmentFromDNSQuery(numDomainLabels, name)
		if seg.Flags.Has(tcpoverdns.FlagMalformed) {
			daemon.logger.Info("handleTCPNameOrOtherQuery", clientIP, nil, "received a malformed TCP-over-DNS segment")
			return
		}
		respSegment, hasResp := daemon.tcpProxy.Receive(seg)
		if !hasResp {
			return
		}
		var err error
		respBody, err = TCPOverDNSSegmentResponse(header, question, respSegment.DNSResource())
		if err != nil {
			daemon.logger.Info("handleTCPNameOrOtherQuery", clientIP, err, "failed to construct DNS query response for TCP-over-DNS segment")
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
		daemon.logger.MaybeMinorError(myForwarder.Close())
	}()
	// Send original query to the resolver without modification
	daemon.logger.MaybeMinorError(myForwarder.SetDeadline(time.Now().Add(ForwarderTimeoutSec * time.Second)))
	if _, err = myForwarder.Write(queryLen); err != nil {
		daemon.logger.Warning("handleTCPRecursiveQuery", clientIP, err, "failed to write length to forwarder")
		return
	} else if _, err = myForwarder.Write(queryBody); err != nil {
		daemon.logger.Warning("handleTCPRecursiveQuery", clientIP, err, "failed to write query to forwarder")
		return
	}
	// Read resolver's response
	respLen := make([]byte, 2)
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
