/*
plainsocket implements a Telnet-comaptible network service to provide unencrypted, plain-text access to all toolbox features.
Due to the unencrypted nature of this communication, users are strongly advised to utilise this service only as a last resort.
The implementation supports UDP as carrier of conversation in addition to TCP.
*/
package plainsocket

import (
	"errors"
	"fmt"
	"net"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

const (
	IOTimeoutSec         = 60               // If a conversation goes silent for this many seconds, the connection is terminated.
	CommandTimeoutSec    = IOTimeoutSec - 1 // Command execution times out after this manys econds
	RateLimitIntervalSec = 1                // Rate limit is calculated at 1 second interval
)

// Daemon implements a Telnet-compatible service to provide unencrypted, plain-text access to all toolbox features, via both TCP and UDP.
type Daemon struct {
	Address    string                   `json:"Address"`    // Network address for both TCP and UDP to listen to, e.g. 0.0.0.0 for all network interfaces.
	TCPPort    int                      `json:"TCPPort"`    // TCP port to listen on
	UDPPort    int                      `json:"UDPPort"`    // UDP port to listen on
	PerIPLimit int                      `json:"PerIPLimit"` // PerIPLimit is approximately how many concurrent users are expected to be using the server from same IP address
	Processor  *common.CommandProcessor `json:"-"`          // Feature command processor

	tcpListener net.Listener    // Once TCP daemon is started, this is its listener.
	udpListener *net.UDPConn    // Once UDP daemon is started, this is its listener.
	rateLimit   *misc.RateLimit // Rate limit counter per IP address
	logger      lalog.Logger    // logger
}

// Initialise validates configuration and initialises internal states.
func (daemon *Daemon) Initialise() error {
	if daemon.Address == "" {
		daemon.Address = "0.0.0.0"
	}
	if daemon.PerIPLimit < 1 {
		daemon.PerIPLimit = 2 // reasonable for personal use
	}
	daemon.logger = lalog.Logger{
		ComponentName: "plainsocket",
		ComponentID:   []lalog.LoggerIDField{{"TCP", daemon.TCPPort}, {"UDP", daemon.UDPPort}},
	}
	if daemon.Processor == nil || daemon.Processor.IsEmpty() {
		return fmt.Errorf("plainsocket.Initialise: command processor and its filters must be configured")
	}
	daemon.Processor.SetLogger(daemon.logger)
	if errs := daemon.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("plainsocket.Initialise: %+v", errs)
	}
	if daemon.UDPPort < 1 && daemon.TCPPort < 1 {
		// No reasonable defaults for these two, sorry.
		return errors.New("plainsocket.Initialise: either or both TCP and UDP ports must be specified and be greater than 0")
	}
	daemon.rateLimit = &misc.RateLimit{
		MaxCount: daemon.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
		Logger:   daemon.logger,
	}
	daemon.rateLimit.Initialise()
	return nil
}

// StartAndBLock starts both TCP and UDP listeners. You may call this function only after having called Initialise().
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
			daemon.logger.Warning("Stop", "", err, "failed to close TCP server")
		}
	}
	if listener := daemon.udpListener; listener != nil {
		if err := listener.Close(); err != nil {
			daemon.logger.Warning("Stop", "", err, "failed to close UDP server")
		}
	}
}
