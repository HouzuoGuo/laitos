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
		logger.Warning("handleTCPQuery", ip, err, "failed to read query length from client")
		return
	}
	queryLenInteger := int(queryLen[0])*256 + int(queryLen[1])
	// Read query packet
	if queryLenInteger > MaxPacketSize || queryLenInteger < MinNameQuerySize {
		logger.Info("handleTCPQuery", ip, nil, "invalid query length from client")
		return
	}
	queryBody := make([]byte, queryLenInteger)
	_, err = conn.Read(queryBody)
	if err != nil {
		logger.Warning("handleTCPQuery", ip, err, "failed to read query from client")
		return
	}
	// Formulate a response
	queryStruct := ParseQueryPacket(queryBody)
	var respBody, respLen []byte
	if queryStruct.IsTextQuery() {
		// Handle toolbox command that arrives as a text query
		respLen, respBody = daemon.handleTCPTextQuery(ip, queryLen, queryBody, queryStruct)
	} else {
		// Handle other query types such as name query
		respLen, respBody = daemon.handleTCPNameOrOtherQuery(ip, queryLen, queryBody, queryStruct)
	}
	// Close client connection in case there is no appropriate response
	if respBody == nil || len(respBody) < 2 {
		return
	}
	// Send response to the client, match transaction ID of original query, the deadline is shared with the read deadline above.
	respBody[0] = queryBody[0]
	respBody[1] = queryBody[1]
	if _, err := conn.Write(respLen); err != nil {
		logger.Warning("handleTCPQuery", ip, err, "failed to answer length to client")
		return
	} else if _, err := conn.Write(respBody); err != nil {
		logger.Warning("handleTCPQuery", ip, err, "failed to answer to client")
		return
	}
}

func (daemon *Daemon) handleTCPTextQuery(clientIP string, queryLen, queryBody []byte, queryStruct *QueryPacket) (respLen, respBody []byte) {
	queriedName := queryStruct.GetHostName()
	if daemon.processQueryTestCaseFunc != nil {
		daemon.processQueryTestCaseFunc(queriedName)
	}
	if dtmfDecoded := DecodeDTMFCommandInput(queryStruct.Labels); len(dtmfDecoded) > 1 {
		cmdResult := daemon.latestCommands.Execute(context.TODO(), daemon.Processor, clientIP, dtmfDecoded)
		if cmdResult.Error == toolbox.ErrPINAndShortcutNotFound {
			/*
				Because the prefix may appear in an ordinary text record query that is not a toolbox command, when there is
				a PIN mismatch, forward to recursive resolver as if the query is indeed not a toolbox command.
			*/
			daemon.logger.Info("handleTCPTextQuery", clientIP, nil, "input has command prefix but failed PIN check, forward to recursive resolver.")
			goto forwardToRecursiveResolver
		} else {
			daemon.logger.Info("handleTCPTextQuery", clientIP, nil, "processed a toolbox command")
			respBody = MakeTextResponse(queryBody, cmdResult.CombinedOutput)
			respLenInt := len(respBody)
			respLen = []byte{byte(respLenInt / 256), byte(respLenInt % 256)}
			return
		}
	} else {
		daemon.logger.Info("handleTCPTextQuery", clientIP, nil, "handle TXT query \"%s\"", string(queriedName))
	}
forwardToRecursiveResolver:
	// There's a chance of being a typo in the PIN entry, make sure this function does not log the request input.
	return daemon.handleTCPRecursiveQuery(clientIP, queryLen, queryBody)
}

func (daemon *Daemon) handleTCPNameOrOtherQuery(clientIP string, queryLen, queryBody []byte, queryStruct *QueryPacket) (respLen, respBody []byte) {
	domainName := queryStruct.GetHostName()
	if domainName == "" {
		daemon.logger.Info("handleTCPNameOrOtherQuery", clientIP, nil, "handle non-name query")
	} else {
		if daemon.processQueryTestCaseFunc != nil {
			daemon.processQueryTestCaseFunc(domainName)
		}
		daemon.logger.Info("handleTCPNameOrOtherQuery", clientIP, nil, "handle IPv%d name query \"%s\"", queryStruct.GetNameQueryVersion(), domainName)
	}
	if daemon.IsInBlacklist(domainName) {
		// Black hole response returns a
		daemon.logger.Info("handleTCPNameOrOtherQuery", clientIP, nil, "handle black-listed IPv%d name query \"%s\"", queryStruct.GetNameQueryVersion(), domainName)
		respBody = GetBlackHoleResponse(queryBody, queryStruct.GetNameQueryVersion() == 6)
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
