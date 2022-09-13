package cli

import (
	"context"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/dnsclient"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
)

func HandleTCPOverDNSClient(logger lalog.Logger, debug bool, port int, proxySegLen int, resolver string, dnsHostName, otpSecret string) {
	if proxySegLen == 0 {
		proxySegLen = dnsclient.OptimalSegLen(dnsHostName)
		logger.Info("", nil, "using segment length %d", proxySegLen)
	}
	httpProxyServer := &dnsclient.Client{
		Address: "127.0.0.1",
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

	if err := httpProxyServer.Initialise(context.Background()); err != nil {
		logger.Panic("", err, "failed to initialise the client http proxy")
		return
	}
	if err := httpProxyServer.StartAndBlock(); err != nil {
		panic(err)
	}
}
