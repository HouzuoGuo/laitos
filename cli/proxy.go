package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
)

// ProxyCLIOptions encapsulates CLI options for the TCP-over-DNS proxy client.
type ProxyCLIOptions struct {
	// Port number of the local HTTP(s) proxy server.
	Port int
	// Debug turns on debug output for both the initiator (local) and responder
	// (remote) transmission control.
	Debug bool
	// EnableDNSRelay starts a recursive resolver on 127.0.0.12:53 to relay
	// DNS queries to laitos DNS server over TCP-over-DNS.
	EnableDNSRelay bool
	// RecursiveResolverAddress is the address of a local or public recursive
	// resolver (ip:port).
	RecursiveResolverAddress string
	// SegmentLenth is the maximum segment length of the initiator's
	// transmission controls.
	MaxSegmentLength int
	// LaitosDNSName is the laitos DNS server's DNS name.
	LaitosDNSName string
	// AccessOTPSecret is the proxy OTP secret for laitos DNS server to
	// authorise this client's connection requests.
	AccessOTPSecret string
	// EnableTXT enables using DNS TXT records in place of CNAME records to
	// carry transmission control segments. TXT records have significantly more
	// capacity and bandwidth.
	EnableTXT bool
	// DownstreamSegmentLength is used for configuring the responder (remote)
	// transmission control's segment length. This enables better utilisation
	// of available bandwidth when the upstream and downstream have asymmetric
	// capacity.
	// If CNAME is used as carrier then the upstream and downstream segment
	// length must be identical. If TXT is used as carrier then the downstream
	// length can be up to ~5 times the upstream length.
	DownstreamSegmentLength int
}

func HandleTCPOverDNSClient(logger lalog.Logger, proxyOpts ProxyCLIOptions) {
	// Initialise the options with default values.
	if proxyOpts.MaxSegmentLength == 0 {
		proxyOpts.MaxSegmentLength = dnsd.MaxUpstreamSegmentLength(proxyOpts.LaitosDNSName)
	}
	if proxyOpts.DownstreamSegmentLength < 1 {
		if proxyOpts.EnableTXT {
			proxyOpts.DownstreamSegmentLength = dnsd.MaxDownstreamSegmentLengthTXT(proxyOpts.LaitosDNSName)
		} else {
			proxyOpts.DownstreamSegmentLength = 1 * proxyOpts.MaxSegmentLength
		}
	}
	logger.Info("", nil, "upstream max segment length: %d, downstream max segment length: %d", proxyOpts.MaxSegmentLength, proxyOpts.DownstreamSegmentLength)

	// Start localhost DNS relay if desired.
	if proxyOpts.EnableDNSRelay {
		go func() {
			relay := &dnsd.DNSRelay{
				Config: tcpoverdns.InitiatorConfig{
					SetConfig:               true,
					Debug:                   proxyOpts.Debug,
					MaxSegmentLenExclHeader: proxyOpts.MaxSegmentLength,
					Timing: tcpoverdns.TimingConfig{
						ReadTimeout:               dnsd.MaxProxyConnectionLifetime,
						WriteTimeout:              dnsd.MaxProxyConnectionLifetime,
						RetransmissionInterval:    5 * time.Second,
						SlidingWindowWaitDuration: 3000 * time.Millisecond,
						// Unlike the HTTP proxy, the timing of DNS relay needs
						// to be a bit tighter to be sufficiently responsive.
						KeepAliveInterval: 1000 * time.Millisecond,
						AckDelay:          100 * time.Millisecond,
					},
				},
				Debug:            proxyOpts.Debug,
				DNSResolver:      proxyOpts.RecursiveResolverAddress,
				DNSHostName:      proxyOpts.LaitosDNSName,
				RequestOTPSecret: proxyOpts.AccessOTPSecret,
				// The port of laitos recursive DNS resolver is hard coded to 53
				// for now.
				ForwardTo: fmt.Sprintf("%s:%d", proxyOpts.LaitosDNSName, 53),
			}
			relayDaemon := &dnsd.Daemon{
				Address:             "127.0.0.12",
				AllowQueryFromCidrs: []string{},
				UDPPort:             53,
				TCPPort:             53,
				DNSRelay:            relay,
			}
			logger.Info(nil, nil, "starting a recursive DNS resolver on %s to relay traffic via TCP-over-DNS to %s", relayDaemon.Address, relay.DNSHostName)
			if err := relayDaemon.Initialise(); err != nil {
				logger.Warning(nil, err, "failed to initialise DNS relay, will proceed to start localhost HTTP server")
				return
			}
			if err := relayDaemon.StartAndBlock(); err != nil {
				logger.Warning(nil, err, "failed to start DNS relay, will proceed to start localhost HTTP server")
			}
		}()

	}

	// Start localhost HTTP proxy server.
	httpProxyServer := &dnsd.HTTPProxyServer{
		Address:                 "127.0.0.12",
		Port:                    proxyOpts.Port,
		EnableTXTRequests:       proxyOpts.EnableTXT,
		DownstreamSegmentLength: proxyOpts.DownstreamSegmentLength,
		Config: tcpoverdns.InitiatorConfig{
			SetConfig:               true,
			Debug:                   proxyOpts.Debug,
			MaxSegmentLenExclHeader: proxyOpts.MaxSegmentLength,
			Timing: tcpoverdns.TimingConfig{
				ReadTimeout:               dnsd.MaxProxyConnectionLifetime,
				WriteTimeout:              dnsd.MaxProxyConnectionLifetime,
				RetransmissionInterval:    7 * time.Second,
				SlidingWindowWaitDuration: 3000 * time.Millisecond,
				KeepAliveInterval:         1500 * time.Millisecond,
				AckDelay:                  500 * time.Millisecond,
			},
		},
		Debug:            proxyOpts.Debug,
		DNSResolver:      proxyOpts.RecursiveResolverAddress,
		DNSHostName:      proxyOpts.LaitosDNSName,
		RequestOTPSecret: proxyOpts.AccessOTPSecret,
	}
	logger.Info(nil, nil, "starting an HTTP (TLS capable) proxy server on %s:%d to relay traffic via TCP-over-DNS to %s", httpProxyServer.Address, httpProxyServer.Port, httpProxyServer.DNSHostName)
	if err := httpProxyServer.Initialise(context.Background()); err != nil {
		logger.Panic("", err, "failed to initialise the client http proxy")
		return
	}
	if err := httpProxyServer.StartAndBlock(); err != nil {
		logger.Panic("", err, "the HTTP proxy server crashed")
	}
}
