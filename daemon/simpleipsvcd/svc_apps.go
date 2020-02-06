package simpleipsvcd

import (
	"net"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

// TCPService implements common.TCPApp interface for a simple IP service.
type TCPService struct {
	// ResponseFun is a function returning a string as the entire response to a simple IP service request.
	ResponseFun func() string
}

// GetTCPStatsCollector returns the stats collector that counts and times client connections for the TCP application.
func (svc *TCPService) GetTCPStatsCollector() *misc.Stats {
	return misc.SimpleIPStatsTCP
}

// HandleTCPConnection
func (svc *TCPService) HandleTCPConnection(logger lalog.Logger, _ string, client *net.TCPConn) {
	logger.MaybeMinorError(client.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second)))
	_, err := client.Write([]byte(svc.ResponseFun() + "\r\n"))
	logger.MaybeMinorError(err)
}

// UDPService implements common.UDPApp interface for a simple IP service.
type UDPService struct {
	// ResponseFun is a function returning a string as the entire response to a simple IP service request.
	ResponseFun func() string
}

// GetUDPStatsCollector returns the stats collector that counts and times client connections for the TCP application.
func (svc *UDPService) GetUDPStatsCollector() *misc.Stats {
	return misc.SimpleIPStatsUDP
}

// HandleTCPConnection
func (svc *UDPService) HandleUDPClient(logger lalog.Logger, _ string, client *net.UDPAddr, _ []byte, srv *net.UDPConn) {
	logger.MaybeMinorError(srv.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second)))
	_, err := srv.WriteToUDP([]byte(svc.ResponseFun()+"\r\n"), client)
	logger.MaybeMinorError(err)
}
