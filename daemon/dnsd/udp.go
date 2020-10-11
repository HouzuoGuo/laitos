package dnsd

import (
	"context"
	"math/rand"
	"net"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"

	"github.com/HouzuoGuo/laitos/misc"
)

// GetUDPStatsCollector returns stats collector for the UDP server of this daemon.
func (daemon *Daemon) GetUDPStatsCollector() *misc.Stats {
	return misc.DNSDStatsUDP
}

// Read a feature command from each input line, then invoke the requested feature and write the execution result back to client.
func (daemon *Daemon) HandleUDPClient(logger lalog.Logger, ip string, client *net.UDPAddr, packet []byte, srv *net.UDPConn) {
	if len(packet) < MinNameQuerySize {
		logger.Warning("HandleUDPClient", ip, nil, "packet length is too small")
		return
	}
	var respLenInt int
	var respBody []byte
	if isTextQuery(packet) {
		// Handle toolbox command that arrives as a text query
		respLenInt, respBody = daemon.handleUDPTextQuery(ip, packet)
	} else {
		// Handle other query types such as name query
		respLenInt, respBody = daemon.handleUDPNameOrOtherQuery(ip, packet)
	}
	// Ignore the request if there is no appropriate response
	if respBody == nil || len(respBody) < 3 {
		return
	}
	// Send response to the client, match transaction ID of original query.
	respBody[0] = packet[0]
	respBody[1] = packet[1]
	// Set deadline for responding to my DNS client because the query reader and response writer do not share the same timeout
	logger.MaybeMinorError(srv.SetWriteDeadline(time.Now().Add(ClientTimeoutSec * time.Second)))
	if _, err := srv.WriteTo(respBody[:respLenInt], client); err != nil {
		logger.Warning("HandleUDPQuery", ip, err, "failed to answer to client")
		return
	}
}

func (daemon *Daemon) handleUDPTextQuery(clientIP string, queryBody []byte) (respLenInt int, respBody []byte) {
	queriedName := ExtractTextQueryInput(queryBody)
	if daemon.processQueryTestCaseFunc != nil {
		daemon.processQueryTestCaseFunc(queriedName)
	}
	if dtmfDecoded := DecodeDTMFCommandInput(queriedName); len(dtmfDecoded) > 1 {
		cmdResult := daemon.latestCommands.Execute(context.TODO(), daemon.Processor, clientIP, dtmfDecoded)
		if cmdResult.Error == toolbox.ErrPINAndShortcutNotFound {
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
		daemon.logger.Info("handleUDPRecursiveQuery", clientIP, nil, "client IP is not allowed to query")
		return
	}
	// Forward the query to a randomly chosen recursive resolver and return its response
	randForwarder := daemon.Forwarders[rand.Intn(len(daemon.Forwarders))]
	forwarderConn, err := net.DialTimeout("udp", randForwarder, ForwarderTimeoutSec*time.Second)
	if err != nil {
		daemon.logger.Warning("handleUDPRecursiveQuery", clientIP, err, "failed to dial forwarder's address")
		return
	}
	daemon.logger.MaybeMinorError(forwarderConn.SetDeadline(time.Now().Add(ForwarderTimeoutSec * time.Second)))
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
