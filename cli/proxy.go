package cli

import (
	"context"
	"time"

	"github.com/HouzuoGuo/laitos/dnsclient"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
)

func HandleTCPOverDNSClient(logger lalog.Logger, debug bool, port int, proxySegLen int, resolverAddr string, resolverPort int, dnsHostName, otpSecret string) {
	if proxySegLen == 0 {
		proxySegLen = dnsclient.OptimalSegLen(dnsHostName)
		logger.Info("HandleTCPOverDNSClient", "", nil, "using segment length %d", proxySegLen)
	}
	httpProxyServer := &dnsclient.Client{
		Address: "127.0.0.1",
		Port:    port,
		Config: tcpoverdns.InitiatorConfig{
			SetConfig:               true,
			Debug:                   debug,
			MaxSegmentLenExclHeader: proxySegLen,
			Timing: tcpoverdns.TimingConfig{
				ReadTimeout:               120 * time.Second,
				WriteTimeout:              120 * time.Second,
				RetransmissionInterval:    15 * time.Second,
				SlidingWindowWaitDuration: 3000 * time.Millisecond,
				KeepAliveInterval:         1500 * time.Millisecond,
				AckDelay:                  500 * time.Millisecond,
			},
		},
		Debug:            debug,
		DNSResolverAddr:  resolverAddr,
		DNSResovlerPort:  resolverPort,
		DNSHostName:      dnsHostName,
		RequestOTPSecret: otpSecret,
	}

	if err := httpProxyServer.Initialise(context.Background()); err != nil {
		logger.Panic("HandleTCPOverDNSClient", "", err, "failed to initialise the client http proxy")
		return
	}
	if err := httpProxyServer.StartAndBlock(); err != nil {
		panic(err)
	}
}
