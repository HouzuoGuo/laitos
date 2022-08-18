package cli

import (
	"context"
	"time"

	"github.com/HouzuoGuo/laitos/dnsclient"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
)

func HandleTCPOverDNSClient(logger lalog.Logger, debug bool, port int, proxySegLen int, resolverAddr string, resolverPort int, dnsHostName string) {
	// There's a ton of overhead in the construction of a DNS response.
	// It takes 16 bytes to encode 3 bytes of arbitrary data in a query
	// answer, and conventionally DNS packets should not exceed 512 bytes in
	// total length - which includes both a repetition of the query and the
	// answer.
	// Some popular public recursive resolvers do not mind handling large
	// UDP query response (e.g. Google public DNS).
	httpProxyServer := &dnsclient.Client{
		Address: "127.0.0.1",
		Port:    port,
		Config: tcpoverdns.InitiatorConfig{
			SetConfig:               true,
			MaxSegmentLenExclHeader: proxySegLen,
			Debug:                   debug,
			Timing: tcpoverdns.TimingConfig{
				ReadTimeout:               120 * time.Second,
				WriteTimeout:              120 * time.Second,
				RetransmissionInterval:    15 * time.Second,
				SlidingWindowWaitDuration: 5 * time.Second,
				KeepAliveInterval:         1500 * time.Millisecond,
				AckDelay:                  500 * time.Millisecond,
			},
		},
		Debug:           debug,
		DNSResolverAddr: resolverAddr,
		DNSResovlerPort: resolverPort,
		DNSHostName:     dnsHostName,
	}

	if err := httpProxyServer.Initialise(context.Background()); err != nil {
		logger.Panic("HandleTCPOverDNSClient", "", err, "failed to initialise the client http proxy")
		return
	}
	if err := httpProxyServer.StartAndBlock(); err != nil {
		panic(err)
	}
}
