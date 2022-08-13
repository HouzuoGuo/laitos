package cli

import (
	"context"

	"github.com/HouzuoGuo/laitos/dnsclient"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
)

func HandleTCPOverDNSClient(logger lalog.Logger, debug bool, port int, resolverAddr string, resolverPort int, dnsHostName string) {
	httpProxyServer := &dnsclient.Client{
		Address: "127.0.0.1",
		Port:    port,
		Config: tcpoverdns.InitiatorConfig{
			SetConfig: true,
			// The max size of DNS query response should be 512 bytes, but the
			// localhost communication does not mind a little extra.
			MaxSegmentLenExclHeader: 120,
			IOTimeoutSec:            100,
			KeepAliveIntervalSec:    1,
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
