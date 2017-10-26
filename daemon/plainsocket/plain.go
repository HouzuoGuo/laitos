package plainsocket

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/misc"
	"net"
)

const (
	IOTimeoutSec         = 60               // If a conversation goes silent for this many seconds, the connection is terminated.
	CommandTimeoutSec    = IOTimeoutSec - 1 // Command execution times out after this manys econds
	RateLimitIntervalSec = 10               // Rate limit is calculated at 10 seconds interval
)

// Daemon provides to features via plain unencrypted TCP and UDP connections.
type Daemon struct {
	Address    string                   `json:"Address"`    // Network address for both TCP and UDP to listen to, e.g. 0.0.0.0 for all network interfaces.
	TCPPort    int                      `json:"TCPPort"`    // TCP port to listen on
	UDPPort    int                      `json:"UDPPort"`    // UDP port to listen on
	PerIPLimit int                      `json:"PerIPLimit"` // How many times in 10 seconds interval a client IP may converse (connect/run feature) with server
	Processor  *common.CommandProcessor `json:"-"`          // Feature command processor

	tcpListener net.Listener    // Once TCP daemon is started, this is its listener.
	udpListener *net.UDPConn    // Once UDP daemon is started, this is its listener.
	rateLimit   *misc.RateLimit // Rate limit counter per IP address
	logger      misc.Logger     // logger
}

// Check configuration and initialise internal states.
func (daemon *Daemon) Initialise() error {
	daemon.logger = misc.Logger{
		ComponentName: "plainsocket",
		ComponentID:   fmt.Sprintf("%s:%d&%d", daemon.Address, daemon.TCPPort, daemon.UDPPort),
	}
	if daemon.Processor == nil || daemon.Processor.IsEmpty() {
		return fmt.Errorf("plainsocket.Initialise: command processor and its filters must be configured")
	}
	daemon.Processor.SetLogger(daemon.logger)
	if errs := daemon.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("plainsocket.Initialise: %+v", errs)
	}
	if daemon.Address == "" {
		return errors.New("plainsocket.Initialise: listen address must not be empty")
	}
	if daemon.UDPPort < 1 && daemon.TCPPort < 1 {
		return errors.New("plainsocket.Initialise: either or both TCP and UDP ports must be specified and be greater than 0")
	}
	if daemon.PerIPLimit < 1 {
		return errors.New("plainsocket.Initialise: PerIPLimit must be greater than 0")
	}
	daemon.rateLimit = &misc.RateLimit{
		MaxCount: daemon.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
		Logger:   daemon.logger,
	}
	daemon.rateLimit.Initialise()
	return nil
}

/*
You may call this function only after having called Initialise()!
Start plain text service on configured TCP and UDP ports. Block caller.
*/
func (daemon *Daemon) StartAndBlock() error {
	numListeners := 0
	errChan := make(chan error, 2)
	if daemon.TCPPort != 0 {
		numListeners++
		go func() {
			err := daemon.StartAndBlockTCP()
			errChan <- err
		}()
	}
	if daemon.UDPPort != 0 {
		numListeners++
		go func() {
			err := daemon.StartAndBlockUDP()
			errChan <- err
		}()
	}
	for i := 0; i < numListeners; i++ {
		if err := <-errChan; err != nil {
			daemon.Stop()
			return err
		}
	}
	return nil
}

// Close all of open TCP and UDP listeners so that they will cease processing incoming connections.
func (daemon *Daemon) Stop() {
	if listener := daemon.tcpListener; listener != nil {
		if err := listener.Close(); err != nil {
			daemon.logger.Warningf("Stop", "", err, "failed to close TCP server")
		}
	}
	if listener := daemon.udpListener; listener != nil {
		if err := listener.Close(); err != nil {
			daemon.logger.Warningf("Stop", "", err, "failed to close UDP server")
		}
	}
}
