package plain

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/global"
	"github.com/HouzuoGuo/laitos/ratelimit"
	"net"
	"time"
)

const (
	IOTimeoutSec         = 120 * time.Second // If a conversation goes silent for this many seconds, the connection is terminated.
	RateLimitIntervalSec = 10                // Rate limit is calculated at 10 seconds interval
)

// Provide access to features via plain unencrypted TCP and UDP connections.
type PlainText struct {
	TCPListenAddress string       `json:"TCPListenAddress"` // TCP network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	TCPListenPort    int          `json:"TCPListenPort"`    // TCP port to listen on
	TCPListener      net.Listener `json:"-"`                // Once TCP daemon is started, this is its listener.

	UDPListenAddress string       `json:"UDPListenAddress"` // UDP network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	UDPListenPort    int          `json:"UDPListenPort"`    // UDP port to listen on
	UDPListener      *net.UDPConn `json:"-"`                // Once UDP daemon is started, this is its listener.

	CommandTimeoutSec int `json:"CommandTimeoutSec"` // Commands get time out error after this number of seconds
	PerIPLimit        int `json:"PerIPLimit"`        // How many times in 10 seconds interval a client IP may converse (connect/run feature) with server

	Processor *common.CommandProcessor `json:"-"` // Feature command processor
	Listener  net.Listener             `json:"-"` // Once daemon is started, this is its TCP listener.
	RateLimit *ratelimit.RateLimit     `json:"-"` // Rate limit counter per IP address
	Logger    global.Logger            `json:"-"` // Logger
}

// Check configuration and initialise internal states.
func (server *PlainText) Initialise() error {
	server.Logger = global.Logger{
		ComponentName: "PlainText",
		ComponentID:   fmt.Sprintf("%s:%d&%s:%d", server.TCPListenAddress, server.TCPListenPort, server.UDPListenAddress, server.UDPListenPort),
	}
	if errs := server.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("PlainText.Initialise: %+v", errs)
	}
	if server.UDPListenAddress == "" && server.TCPListenAddress == "" {
		return errors.New("PlainText.Initialise: listen address must not be empty")
	}
	if server.UDPListenPort < 1 && server.TCPListenPort < 1 {
		return errors.New("PlainText.Initialise: listen port must be greater than 0")
	}
	server.RateLimit = &ratelimit.RateLimit{
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
func (server *PlainText) StartAndBlock() error {
	numListeners := 0
	errChan := make(chan error, 2)
	if server.TCPListenPort != 0 {
		numListeners++
		go func() {
			err := server.StartAndBlockTCP()
			errChan <- err
		}()
	}
	if server.UDPListenPort != 0 {
		numListeners++
		go func() {
			err := server.StartAndBlockUDP()
			errChan <- err
		}()
	}
	if numListeners == 0 {
		return fmt.Errorf("PlainText.StartAndBlock: neither UDP nor TCP listen port is defined, the daemon will not start.")
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
func (server *PlainText) Stop() {
	if listener := server.TCPListener; listener != nil {
		if err := listener.Close(); err != nil {
			server.Logger.Warningf("Stop", "", err, "failed to close TCP listener")
		}
	}
	if listener := server.UDPListener; listener != nil {
		if err := listener.Close(); err != nil {
			server.Logger.Warningf("Stop", "", err, "failed to close UDP listener")
		}
	}
}
