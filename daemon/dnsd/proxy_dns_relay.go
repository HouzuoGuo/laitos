package dnsd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/miekg/dns"
)

type DNSRelay struct {
	// Config contains the parameters for the initiator of the proxy
	// connections to configure the remote transmission control.
	Config tcpoverdns.InitiatorConfig
	// Debug enables verbose logging for IO activities.
	Debug bool
	// RequestOTPSecret is a TOTP secret for authorising outgoing connection
	// requests.
	RequestOTPSecret string

	// DNSResolver is the address (ip:port) of the public recursive DNS resolver.
	DNSResolver string
	// DNSHostName is the host name of the TCP-over-DNS proxy server.
	DNSHostName string
	dnsConfig   *dns.ClientConfig

	// ForwardTo is the address (ip:port) of the public recursive DNS resolver.
	ForwardTo string
	// TransactionMutex is for relay's client to obtain to ensure proper
	// serialisation of DNS request-response transactions.
	TransactionMutex *sync.Mutex

	mutex             *sync.Mutex
	proxiedConnection *ProxiedConnection
	logger            *lalog.Logger
	context           context.Context
	cancelFun         func()
}

// Initialise validates configuration parameters and initialises the internal
// state of the relay.
func (relay *DNSRelay) Initialise(ctx context.Context) error {
	relay.mutex = new(sync.Mutex)
	relay.TransactionMutex = new(sync.Mutex)
	if len(relay.DNSHostName) < 3 {
		return fmt.Errorf("DNSDomainName (%q) must be a valid host name", relay.DNSHostName)
	}
	if relay.DNSHostName[0] == '.' {
		relay.DNSHostName = relay.DNSHostName[1:]
	}
	relay.logger = &lalog.Logger{ComponentName: "DNSRelay", ComponentID: []lalog.LoggerIDField{{Key: "ForwardTo", Value: relay.ForwardTo}}}
	relay.context, relay.cancelFun = context.WithCancel(ctx)

	var err error
	if relay.DNSResolver == "" {
		relay.dnsConfig, err = dns.ClientConfigFromFile("/etc/resolv.conf")
		if err != nil {
			return err
		}
		if len(relay.dnsConfig.Servers) == 0 {
			return fmt.Errorf("resolv.conf appears to be malformed or empty, try specifying an explicit DNS resolver address instead.")
		}
	} else {
		host, port, err := net.SplitHostPort(relay.DNSResolver)
		if err != nil {
			return fmt.Errorf("failed to parse ip:port from DNS resolver %q", err)
		}
		portInt, err := strconv.Atoi(port)
		if err != nil {
			return fmt.Errorf("failed to parse ip:port from DNS resolver %q", err)
		}
		relay.dnsConfig = &dns.ClientConfig{
			Servers: []string{host},
			Port:    strconv.Itoa(portInt),
		}
	}
	return nil
}

func (relay *DNSRelay) establish(ctx context.Context) (*ProxiedConnection, error) {
	_, curr, _, err := toolbox.GetTwoFACodes(relay.RequestOTPSecret)
	if err != nil {
		return nil, err
	}
	initiatorSegment, err := json.Marshal(ProxyRequest{
		Network:    "tcp",
		Address:    relay.ForwardTo,
		AccessTOTP: curr,
	})
	if err != nil {
		return nil, err
	}
	tcID := uint16(rand.Int())
	proxyServerIn, inTransport := net.Pipe()
	relay.logger.Info(fmt.Sprint(tcID), nil, "creating transmission control for %s", string(initiatorSegment))
	tc := &tcpoverdns.TransmissionControl{
		LogTag:               "DNSRelay",
		ID:                   tcID,
		Debug:                relay.Debug,
		InitiatorSegmentData: initiatorSegment,
		InitiatorConfig:      relay.Config,
		Initiator:            true,
		InputTransport:       inTransport,
		MaxLifetime:          MaxProxyConnectionLifetime,
		// In practice there are occasionally bursts of tens of errors at a
		// time before recovery.
		MaxTransportErrors: 300,
		// The duration of all retransmissions (if all go unacknowledged) is
		// MaxRetransmissions x SlidingWindowWaitDuration.
		MaxRetransmissions: 300,
		// The output transport is not used. Instead, the output segments
		// are kept in a backlog.
		OutputTransport: io.Discard,
	}
	relay.Config.Config(tc)
	conn := &ProxiedConnection{
		debug:       relay.Debug,
		dnsHostName: relay.DNSHostName,
		dnsConfig:   relay.dnsConfig,
		in:          proxyServerIn,
		tc:          tc,
		context:     ctx,
		logger: &lalog.Logger{
			ComponentName: "DNSClientProxyConn",
			ComponentID: []lalog.LoggerIDField{
				{Key: "TCID", Value: tc.ID},
			},
		},
	}
	return conn, conn.Start()
}

// TransmissionControl waits for the proxied connection's transmission control
// to reach the establish state and then returns it.
func (relay *DNSRelay) TransmissionControl(ctx context.Context) *tcpoverdns.TransmissionControl {
	for {
		relay.mutex.Lock()
		if conn := relay.proxiedConnection; conn != nil {
			if tc := conn.tc; tc.State() == tcpoverdns.StateEstablished {
				relay.mutex.Unlock()
				return tc
			}
		}
		relay.mutex.Unlock()
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(tcpoverdns.BusyWaitInterval):
			continue
		}
	}
}

// StartAndBlock starts the internal transmission control to act as a relay
// toward the DNS forwarder.
func (relay *DNSRelay) StartAndBlock() error {
	relay.logger.Info("", nil, "starting now")
	for {
		var proxyConn *ProxiedConnection
		var err error
		relay.mutex.Lock()
		proxyConn, err = relay.establish(relay.context)
		relay.proxiedConnection = proxyConn
		relay.mutex.Unlock()
		if err != nil {
			relay.Stop()
			return err
		}
		// When the transmission control closes, re-establish the transmission
		// control.
		// The TCP query handler of DNS daemon always closes the connection
		// after every request.
		if !proxyConn.tc.WaitState(relay.context, tcpoverdns.StateClosed) {
			return nil
		}
	}
}

// Stop the relay.
func (relay *DNSRelay) Stop() {
	relay.cancelFun()
}
