package plainsockets

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/misc"
	"net"
)

const (
	IOTimeoutSec         = 120              // If a conversation goes silent for this many seconds, the connection is terminated.
	CommandTimeoutSec    = IOTimeoutSec - 1 // Command execution times out after this manys econds
	RateLimitIntervalSec = 10               // Rate limit is calculated at 10 seconds interval
)

// Daemon provides to features via plain unencrypted TCP and UDP connections.
type Daemon struct {
	Address     string       `json:"Address"` // Network address for both TCP and UDP to listen to, e.g. 0.0.0.0 for all network interfaces.
	TCPPort     int          `json:"TCPPort"` // TCP port to listen on
	UDPPort     int          `json:"UDPPort"` // UDP port to listen on
	TCPListener net.Listener `json:"-"`       // Once TCP daemon is started, this is its listener.
	UDPListener *net.UDPConn `json:"-"`       // Once UDP daemon is started, this is its listener.

	PerIPLimit int `json:"PerIPLimit"` // How many times in 10 seconds interval a client IP may converse (connect/run feature) with server

	Processor *common.CommandProcessor `json:"-"` // Feature command processor
	RateLimit *misc.RateLimit          `json:"-"` // Rate limit counter per IP address
	Logger    misc.Logger              `json:"-"` // Logger
}

// Check configuration and initialise internal states.
func (server *Daemon) Initialise() error {
	server.Logger = misc.Logger{
		ComponentName: "plainsockets",
		ComponentID:   fmt.Sprintf("%s:%d&%d", server.Address, server.TCPPort, server.UDPPort),
	}
	if server.Processor == nil {
		server.Processor = common.GetEmptyCommandProcessor()
	}
	server.Processor.SetLogger(server.Logger)
	if errs := server.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("plainsockets.Initialise: %+v", errs)
	}
	if server.Address == "" {
		return errors.New("plainsockets.Initialise: listen address must not be empty")
	}
	if server.UDPPort < 1 && server.TCPPort < 1 {
		return errors.New("plainsockets.Initialise: either or both TCP and UDP ports must be specified and be greater than 0")
	}
	if server.PerIPLimit < 1 {
		return errors.New("plainsockets.Initialise: PerIPLimit must be greater than 0")
	}
	server.RateLimit = &misc.RateLimit{
		MaxCount: server.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
		Logger:   server.Logger,
	}
	server.RateLimit.Initialise()
	return nil
}

/*
You may call this function only after having called Initialise()!
Start plain text service on configured TCP and UDP ports. Block caller.
*/
func (server *Daemon) StartAndBlock() error {
	numListeners := 0
	errChan := make(chan error, 2)
	if server.TCPPort != 0 {
		numListeners++
		go func() {
			err := server.StartAndBlockTCP()
			errChan <- err
		}()
	}
	if server.UDPPort != 0 {
		numListeners++
		go func() {
			err := server.StartAndBlockUDP()
			errChan <- err
		}()
	}
	for i := 0; i < numListeners; i++ {
		if err := <-errChan; err != nil {
			server.Stop()
			return err
		}
	}
	return nil
}

// Close all of open TCP and UDP listeners so that they will cease processing incoming connections.
func (server *Daemon) Stop() {
	if listener := server.TCPListener; listener != nil {
		if err := listener.Close(); err != nil {
			server.Logger.Warningf("Stop", "", err, "failed to close TCP server")
		}
	}
	if listener := server.UDPListener; listener != nil {
		if err := listener.Close(); err != nil {
			server.Logger.Warningf("Stop", "", err, "failed to close UDP server")
		}
	}
}
