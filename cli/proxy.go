package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
)

func HandleTCPOverDNSClient(logger lalog.Logger, debug, relayDNS bool, port int, proxySegLen int, resolver string, dnsHostName, otpSecret string) {
	if proxySegLen == 0 {
		proxySegLen = dnsd.OptimalSegLen(dnsHostName)
		logger.Info("", nil, "using segment length %d", proxySegLen)
	}

	// Start localhost DNS relay if desired.
	if relayDNS {
		go func() {
			relay := &dnsd.DNSRelay{
				Config: tcpoverdns.InitiatorConfig{
					SetConfig:               true,
					Debug:                   debug,
					MaxSegmentLenExclHeader: proxySegLen,
					Timing: tcpoverdns.TimingConfig{
						ReadTimeout:               dnsd.MaxProxyConnectionLifetime,
						WriteTimeout:              dnsd.MaxProxyConnectionLifetime,
						RetransmissionInterval:    5 * time.Second,
						SlidingWindowWaitDuration: 3000 * time.Millisecond,
						// Unlike the HTTP proxy, the timing of DNS relay needs to be a
						// bit tighter to be sufficiently responsive.
						KeepAliveInterval: 1000 * time.Millisecond,
						AckDelay:          100 * time.Millisecond,
					},
				},
				Debug:            true,
				DNSResolver:      resolver,
				DNSHostName:      dnsHostName,
				RequestOTPSecret: otpSecret,
				// The port of laitos recursive DNS resolver is hard coded to 53
				// for now.
				ForwardTo: fmt.Sprintf("%s:%d", dnsHostName, 53),
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
		Address: "127.0.0.12",
		Port:    port,
		Config: tcpoverdns.InitiatorConfig{
			SetConfig:               true,
			Debug:                   debug,
			MaxSegmentLenExclHeader: proxySegLen,
			Timing: tcpoverdns.TimingConfig{
				ReadTimeout:               dnsd.MaxProxyConnectionLifetime,
				WriteTimeout:              dnsd.MaxProxyConnectionLifetime,
				RetransmissionInterval:    5 * time.Second,
				SlidingWindowWaitDuration: 3000 * time.Millisecond,
				KeepAliveInterval:         1500 * time.Millisecond,
				AckDelay:                  500 * time.Millisecond,
			},
		},
		Debug:            debug,
		DNSResolver:      resolver,
		DNSHostName:      dnsHostName,
		RequestOTPSecret: otpSecret,
	}
	logger.Info(nil, nil, "starting an HTTP (TLS capable) proxy server on %s:%d to relay traffic via TCP-over-DNS to %s", httpProxyServer.Address, port, httpProxyServer.DNSHostName)
	if err := httpProxyServer.Initialise(context.Background()); err != nil {
		logger.Panic("", err, "failed to initialise the client http proxy")
		return
	}
	if err := httpProxyServer.StartAndBlock(); err != nil {
		logger.Panic("", err, "the HTTP proxy server crashed")
	}
}
