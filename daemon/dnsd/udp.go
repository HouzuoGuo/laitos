package dnsd

import (
	"context"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
	"golang.org/x/net/dns/dnsmessage"

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
	// Parse the first (and only) query question.
	parser := new(dnsmessage.Parser)
	header, err := parser.Start(packet)
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
		respBody = daemon.handleUDPTextQuery(ip, packet, header, question)
	} else {
		// Handle all other query types.
		respBody = daemon.handleUDPNameOrOtherQuery(ip, packet, header, question)
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
		logger.Warning("HandleUDPQuery", ip, err, "failed to answer to client")
		return
	}
}

func (daemon *Daemon) handleUDPTextQuery(clientIP string, queryBody []byte, header dnsmessage.Header, question dnsmessage.Question) (respBody []byte) {
	name := question.Name.String()
	if daemon.processQueryTestCaseFunc != nil {
		daemon.processQueryTestCaseFunc(name)
	}
	// Remove the domain name from the labels.
	labelsWithoutDomain := strings.Split(name, ".")
	if len(labelsWithoutDomain) > 2 {
		labelsWithoutDomain = labelsWithoutDomain[:len(labelsWithoutDomain)-2]
	}
	if dtmfDecoded := DecodeDTMFCommandInput(labelsWithoutDomain); len(dtmfDecoded) > 1 {
		cmdResult := daemon.latestCommands.Execute(context.Background(), daemon.Processor, clientIP, dtmfDecoded)
		if cmdResult.Error == toolbox.ErrPINAndShortcutNotFound {
			// Because the prefix may appear in an ordinary text record query
			// that is not a toolbox command, when there is a PIN mismatch,
			// forward to recursive resolver as if the query is indeed not a
			// toolbox command.
			daemon.logger.Info("handleUDPTextQuery", clientIP, nil, "the queried name has the command prefix but failed PIN check, forwarding to recursive resolver.")
			goto forwardToRecursiveResolver
		} else {
			var err error
			daemon.logger.Info("handleUDPTextQuery", clientIP, nil, "processed a toolbox command")
			respBody, err = BuildTextResponse(name, header, question, []string{cmdResult.CombinedOutput})
			if err != nil {
				daemon.logger.Warning("handleUDPTextQuery", clientIP, err, "failed to build response packet")
				return nil
			}
			return respBody
		}
	} else {
		daemon.logger.Info("handleUDPTextQuery", clientIP, nil, "handle TXT query %q", name)
	}
forwardToRecursiveResolver:
	// Because the password PIN in the query may contain a typo, make sure this
	// function does not incidentally log the query name.
	return daemon.handleUDPRecursiveQuery(clientIP, queryBody)
}

func (daemon *Daemon) handleUDPNameOrOtherQuery(clientIP string, queryBody []byte, header dnsmessage.Header, question dnsmessage.Question) (respBody []byte) {
	name := question.Name.String()
	if name == "" {
		daemon.logger.Info("handleUDPNameOrOtherQuery", clientIP, nil, "handle non-name query")
	} else {
		if daemon.processQueryTestCaseFunc != nil {
			daemon.processQueryTestCaseFunc(name)
		}
		daemon.logger.Info("handleUDPNameOrOtherQuery", clientIP, nil, "handle name query %q", name)
	}
	if daemon.IsInBlacklist(name) {
		daemon.logger.Info("handleUDPNameOrOtherQuery", clientIP, nil, "handle black-listed name query %q", name)
		var err error
		respBody, err = BuildBlackHoleAddrResponse(header, question)
		if err != nil {
			daemon.logger.Warning("handleTCPNameOrOtherQuery", clientIP, err, "failed to build response packet")
			return nil
		}
	} else {
		respBody = daemon.handleUDPRecursiveQuery(clientIP, queryBody)
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
	defer func() {
		daemon.logger.MaybeMinorError(forwarderConn.Close())
	}()
	daemon.logger.MaybeMinorError(forwarderConn.SetDeadline(time.Now().Add(ForwarderTimeoutSec * time.Second)))
	if _, err := forwarderConn.Write(queryBody); err != nil {
		daemon.logger.Warning("handleUDPRecursiveQuery", clientIP, err, "failed to write to forwarder")
		return
	}
	respBody = make([]byte, MaxPacketSize)
	respLenInt, err := forwarderConn.Read(respBody)
	if err != nil {
		daemon.logger.Warning("handleUDPRecursiveQuery", clientIP, err, "failed to read from forwarder")
		return
	}
	if respLenInt < 3 {
		daemon.logger.Warning("handleUDPRecursiveQuery", clientIP, err, "forwarder response is abnormally small")
		return
	}
	respBody = respBody[:respLenInt]
	return
}
