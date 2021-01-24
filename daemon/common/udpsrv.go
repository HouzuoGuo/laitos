package common

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

const (
	// MaxPacketSize is the maximum acceptable size for a single UDP packet
	MaxUDPPacketSize = 9038
)

// UDPApp defines routines for a UDP server to read, process, and interact with UDP clients.
type UDPApp interface {
	// GetUDPStatsCollector returns the stats collector that counts and times UDP conversations.
	GetUDPStatsCollector() *misc.Stats
	// HandleUDPClient converses with a UDP client based on a received packet.
	HandleUDPClient(lalog.Logger, string, *net.UDPAddr, []byte, *net.UDPConn)
}

// UDPServer implements common routines for a UDP server that interacts with unlimited number of clients while applying a rate limit.
type UDPServer struct {
	// ListenAddr is the IP address to listen on. Use 0.0.0.0 to listen on all network interfaces.
	ListenAddr string
	// ListenPort is the port number to listen on.
	ListenPort int
	// AppName is a human readable name that identifies the server application in log entries.
	AppName string
	// App is the concrete implementation of UDP server application.
	App UDPApp
	/*
		LimitPerSec is the maximum number of actions and connections acceptable from a single IP at a time.
		Once the limit is reached, new connections from the IP will be closed right away, and existing conversations are
		terminated.
	*/
	LimitPerSec int

	mutex     *sync.Mutex
	logger    lalog.Logger
	rateLimit *misc.RateLimit
	udpServer *net.UDPConn
}

// NewUDPServer constructs a new UDP server and initialises its internal structures.
func NewUDPServer(listenAddr string, listenPort int, appName string, app UDPApp, limitPerSec int) (srv *UDPServer) {
	srv = &UDPServer{
		ListenAddr:  listenAddr,
		ListenPort:  listenPort,
		AppName:     appName,
		App:         app,
		LimitPerSec: limitPerSec,
	}
	srv.Initialise()
	return
}

// Initialise initialises the internal structures of UDP server, preparing it for processing clients.
func (srv *UDPServer) Initialise() {
	srv.mutex = new(sync.Mutex)
	srv.logger = lalog.Logger{
		ComponentName: srv.AppName,
		ComponentID:   []lalog.LoggerIDField{{Key: "Addr", Value: srv.ListenAddr}, {Key: "UDPPort", Value: srv.ListenPort}},
	}
	srv.rateLimit = &misc.RateLimit{
		UnitSecs: 1,
		MaxCount: srv.LimitPerSec,
		Logger:   srv.logger,
	}
	srv.rateLimit = &misc.RateLimit{Logger: srv.logger, UnitSecs: 1, MaxCount: srv.LimitPerSec}
	srv.rateLimit.Initialise()
}

/*
StartAndBlock starts UDP listener to process clients and blocks until the server is told to stop.
Call this function after having initialised the UDP server.
*/
func (srv *UDPServer) StartAndBlock() error {
	defer srv.Stop()
	srv.mutex.Lock()
	if srv.udpServer != nil {
		srv.mutex.Unlock()
		return fmt.Errorf("UDPServer.StartAndBlock(%s): listener on port %d must not be started a second time", srv.AppName, srv.ListenPort)
	}
	srv.logger.Info("StartAndBlock", "", nil, "starting UDP listener")
	var err error
	listenUDPAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(srv.ListenAddr, strconv.Itoa(srv.ListenPort)))
	if err != nil {
		return fmt.Errorf("UDPServer.StartAndBlock(%s): failed to resolve listning address %s - %v", srv.AppName, srv.ListenAddr, err)
	}
	srv.udpServer, err = net.ListenUDP("udp", listenUDPAddr)
	srv.mutex.Unlock()
	if err != nil {
		return fmt.Errorf("UDPServer.StartAndBlock(%s): failed to listen on port %d - %v", srv.AppName, srv.ListenPort, err)
	}
	packet := make([]byte, MaxUDPPacketSize)
	for {
		if misc.EmergencyLockDown {
			srv.logger.Warning("StartAndBlock", srv.AppName, misc.ErrEmergencyLockDown, "")
			return misc.ErrEmergencyLockDown
		}
		packetLen, clientAddr, err := srv.udpServer.ReadFromUDP(packet)
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			return fmt.Errorf("UDPServer.StartAndBlock(%s): failed to read from next client - %v", srv.AppName, err)
		}
		if packetLen == 0 {
			continue
		}
		// Check client IP against rate limit
		clientIP := clientAddr.IP.String()
		if !srv.rateLimit.Add(clientIP, true) {
			continue
		}
		// Make a copy of the packet for processing because multiple packets may be processed concurrently
		packetCopy := make([]byte, packetLen)
		copy(packetCopy, packet[:packetLen])
		go srv.handleClient(srv.udpServer, clientIP, clientAddr, packetCopy)
	}
}

// AddAndCheckRateLimit may be optionally invoked by UDP application in the middle of an ongoing conversation to check whether conversation is going on too fast.
func (srv *UDPServer) AddAndCheckRateLimit(clientIP string) bool {
	return srv.rateLimit.Add(clientIP, true)
}

// handleConnection is launched in an independent goroutine by StartAndBlock to interact with a connected client.
func (srv *UDPServer) handleClient(udpServer *net.UDPConn, clientIP string, clientAddr *net.UDPAddr, packet []byte) {
	if udpServer == nil {
		// Server has already shut down
		return
	}
	// Put processing duration into statistics
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		srv.App.GetUDPStatsCollector().Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()
	srv.logger.Info("handleClient", clientIP, nil, "conversation started")
	// Apply the default IO timeout to prevent a potentially malfunctioning connection handler from hanging
	if err := udpServer.SetWriteDeadline(time.Now().Add(ServerDefaultIOTimeoutSec * time.Second)); err != nil {
		srv.logger.Warning("handleClient", clientIP, err, "failed to set default write deadline, terminating the conversation.")
		return
	}
	srv.App.HandleUDPClient(srv.logger, clientIP, clientAddr, packet, udpServer)
}

// IsRunning returns true only if the server has started and has not been told to stop.
func (srv *UDPServer) IsRunning() bool {
	srv.mutex.Lock()
	defer srv.mutex.Unlock()
	return srv.udpServer != nil
}

// Stop the UDP server from accepting new clients. Ongoing conversations will continue nonetheless.
func (srv *UDPServer) Stop() {
	srv.mutex.Lock()
	defer srv.mutex.Unlock()
	if srv.udpServer != nil {
		if err := srv.udpServer.Close(); err != nil {
			srv.logger.Warning("Stop", srv.AppName, err, "failed to stop UDP server listener")
		}
		srv.udpServer = nil
	}
	srv.logger.Info("Stop", "", nil, "UDP server has shut down successfully")
}
