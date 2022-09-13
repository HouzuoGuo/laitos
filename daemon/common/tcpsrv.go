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
	// ServerRateLimitIntervalSec is the interval at which client rate limit counter operates, i.e. maximum N clients per interval of X.
	ServerRateLimitIntervalSec = 1
	/*
		ServerDefaultIOTimeoutSec is the default IO timeout applied to all client connections. The IO timeout prevents a
		potentially malfunctioning server application from hanging at a lingering client.
		Server application should always override the default IO timeout by setting a new timeout in connection handler.
	*/
	ServerDefaultIOTimeoutSec = 10 * 60
)

// TCPApp defines routines for a TCP server application to accept, process, and interact with client connections.
type TCPApp interface {
	// GetTCPStatsCollector returns the stats collector that counts and times client connections for the TCP application.
	GetTCPStatsCollector() *misc.Stats
	// HandleTCPConnection converses with the TCP client. The client connection is closed by server upon returning from the implementation.
	HandleTCPConnection(lalog.Logger, string, *net.TCPConn)
}

// TCPServer implements common routines for a TCP server that interacts with unlimited number of clients while applying a rate limit.
type TCPServer struct {
	// ListenAddr is the IP address to listen on. Use 0.0.0.0 to listen on all network interfaces.
	ListenAddr string
	// ListenPort is the port number to listen on.
	ListenPort int
	// AppName is a human readable name that identifies the server application in log entries.
	AppName string
	// App is the concrete implementation of TCP server application.
	App TCPApp
	/*
		LimitPerSec is the maximum number of actions and connections acceptable from a single IP at a time.
		Once the limit is reached, new connections from the IP will be closed right away, and existing conversations are
		terminated.
	*/
	LimitPerSec int

	mutex     *sync.Mutex
	logger    lalog.Logger
	rateLimit *misc.RateLimit
	listener  net.Listener
}

// NewTCPServer constructs a new TCP server and initialises its internal structures.
func NewTCPServer(listenAddr string, listenPort int, appName string, app TCPApp, limitPerSec int) (srv *TCPServer) {
	srv = &TCPServer{
		ListenAddr:  listenAddr,
		ListenPort:  listenPort,
		AppName:     appName,
		App:         app,
		LimitPerSec: limitPerSec,
	}
	srv.Initialise()
	return
}

// Initialise initialises the internal structures of the TCP server, preparing it for accepting clients.
func (srv *TCPServer) Initialise() {
	srv.mutex = new(sync.Mutex)
	srv.logger = lalog.Logger{
		ComponentName: srv.AppName,
		ComponentID:   []lalog.LoggerIDField{{Key: "Addr", Value: srv.ListenAddr}, {Key: "TCPPort", Value: srv.ListenPort}},
	}
	srv.rateLimit = &misc.RateLimit{Logger: srv.logger, UnitSecs: 1, MaxCount: srv.LimitPerSec}
	srv.rateLimit.Initialise()
}

/*
StartAndBlock starts TCP listener to process client connections and blocks until the server is told to stop.
Call this function after having initialised the TCP server.
*/
func (srv *TCPServer) StartAndBlock() error {
	defer srv.Stop()
	srv.mutex.Lock()
	if srv.listener != nil {
		srv.mutex.Unlock()
		return fmt.Errorf("TCPServer.StartAndBlock(%s): listener on port %d must not be started a second time", srv.AppName, srv.ListenPort)
	}
	srv.logger.Info("", nil, "starting TCP listener")
	var err error
	listener, err := net.Listen("tcp", net.JoinHostPort(srv.ListenAddr, strconv.Itoa(srv.ListenPort)))
	if err != nil {
		srv.mutex.Unlock()
		return fmt.Errorf("TCPServer.StartAndBlock(%s): failed to listen on port %d - %v", srv.AppName, srv.ListenPort, err)
	}
	srv.listener = listener
	srv.mutex.Unlock()
	for {
		if misc.EmergencyLockDown {
			srv.logger.Warning(srv.AppName, misc.ErrEmergencyLockDown, "")
			return misc.ErrEmergencyLockDown
		}
		client, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			return fmt.Errorf("TCPServer.StartAndBlock(%s): failed to accept new connection - %v", srv.AppName, err)
		}
		// Check client IP against rate limit
		tcpClient := client.(*net.TCPConn)
		clientIP := tcpClient.RemoteAddr().(*net.TCPAddr).IP.String()
		if !srv.rateLimit.Add(clientIP, true) {
			srv.logger.MaybeMinorError(tcpClient.Close())
			continue
		}
		go srv.handleConnection(clientIP, tcpClient)
	}
}

// AddAndCheckRateLimit may be optionally invoked by TCP application in the middle of an ongoing conversation to check whether conversation is going on too fast.
func (srv *TCPServer) AddAndCheckRateLimit(clientIP string) bool {
	return srv.rateLimit.Add(clientIP, true)
}

// handleConnection is launched in an independent goroutine by StartAndBlock to interact with a connected client.
func (srv *TCPServer) handleConnection(clientIP string, client *net.TCPConn) {
	// Put processing duration into statistics
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		srv.logger.MaybeMinorError(client.Close())
		srv.App.GetTCPStatsCollector().Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()
	srv.logger.Info(clientIP, nil, "connection is accepted")
	// Turn on keep-alive for OS to detect and remove dead clients
	if err := client.SetKeepAlive(true); err != nil {
		srv.logger.Warning(clientIP, err, "failed to turn on keep alive")
	}
	if err := client.SetKeepAlivePeriod(ServerDefaultIOTimeoutSec / 3); err != nil {
		srv.logger.Warning(clientIP, err, "failed to turn on keep alive")
	}
	// Apply the default IO timeout to prevent a potentially malfunctioning connection handler from hanging
	if err := client.SetReadDeadline(time.Now().Add(ServerDefaultIOTimeoutSec * time.Second)); err != nil {
		srv.logger.Warning(clientIP, err, "failed to set default read deadline, terminating the connection.")
		return
	}
	if err := client.SetWriteDeadline(time.Now().Add(ServerDefaultIOTimeoutSec * time.Second)); err != nil {
		srv.logger.Warning(clientIP, err, "failed to set default write deadline, terminating the connection.")
		return
	}
	srv.App.HandleTCPConnection(srv.logger, clientIP, client)
}

// Stop the TCP server from accepting new connections. Ongoing connections will continue nonetheless.
func (srv *TCPServer) Stop() {
	srv.mutex.Lock()
	defer srv.mutex.Unlock()
	if srv.listener != nil {
		if err := srv.listener.Close(); err != nil {
			srv.logger.Warning(srv.AppName, err, "failed to stop TCP server listener")
		}
		srv.listener = nil
	}
	srv.logger.Info(srv.AppName, nil, "TCP server has shut down successfully")
}
