package dnsd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/httpd/middleware"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/miekg/dns"
)

// HTTPProxyServer is an HTTP proxy server that tunnels its HTTP clients'
// traffic through TCP-over-DNS proxy.
type HTTPProxyServer struct {
	// Address is the network address for the HTTP proxy to listen on, e.g.
	// 127.0.0.1 to serve the localhost alone.
	Address string `json:"Address"`
	// Port to listen on.
	Port int `json:"Port"`
	// Config contains the parameters for the initiator of the proxy
	// connections to configure the remote transmission control.
	Config          tcpoverdns.InitiatorConfig
	responderConfig tcpoverdns.InitiatorConfig
	// Debug enables verbose logging for IO activities.
	Debug bool
	// EnableTXTRequests forces the DNS client to transport TCP-over-DNS
	// segments in TXT queries instead of the usual CNAME queries.
	EnableTXTRequests bool
	// DownstreamSegmentLength is used for configuring the responder (remote)
	// transmission control's segment length. This enables better utilisation
	// of available bandwidth when the upstream and downstream have asymmetric
	// capacity.
	DownstreamSegmentLength int
	// RequestOTPSecret is the proxy OTP secret for laitos DNS server to
	// authorise this client's connection requests.
	RequestOTPSecret string `json:"RequestOTPSecret"`

	// httpTransport is the HTTP round tripper used by the proxy handler for
	// HTTP (unencrypted) proxy requests. This transport is not used for handling
	// HTTPS (HTTP CONNECT) requests.
	httpTransport *http.Transport

	// DNSResolver is the address of a local or public recursive resolver
	// (ip:port).
	DNSResolver string
	// DNSHostName is the host name of the TCP-over-DNS proxy server.
	DNSHostName string

	dnsConfig *dns.ClientConfig
	// dropPercentage is the percentage of resposnes to be dropped (returned as
	// error). This is for internal testing only.
	dropPercentage             int
	proxyHandlerWithMiddleware http.HandlerFunc
	logger                     *lalog.Logger
	httpServer                 *http.Server
	context                    context.Context
	cancelFun                  func()
}

// Initialise validates configuration parameters and initialises the internal state of the daemon.
func (proxy *HTTPProxyServer) Initialise(ctx context.Context) error {
	if proxy.Address == "" {
		proxy.Address = "127.0.0.1"
	}
	if proxy.Port == 0 {
		proxy.Port = 8080
	}
	if len(proxy.DNSHostName) < 3 {
		return fmt.Errorf("DNSDomainName (%q) must be a valid host name", proxy.DNSHostName)
	}
	if proxy.DNSHostName[0] == '.' {
		proxy.DNSHostName = proxy.DNSHostName[1:]
	}
	proxy.logger = &lalog.Logger{ComponentName: "HTTPProxyServer", ComponentID: []lalog.LoggerIDField{{Key: "Port", Value: strconv.Itoa(proxy.Port)}}}
	proxy.proxyHandlerWithMiddleware = middleware.LogRequestStats(proxy.logger, middleware.EmergencyLockdown(proxy.ProxyHandler))
	proxy.context, proxy.cancelFun = context.WithCancel(ctx)

	proxy.httpTransport = &http.Transport{
		Proxy:                 nil,
		DialContext:           proxy.dialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          100,
		IdleConnTimeout:       proxy.Config.Timing.ReadTimeout,
		TLSHandshakeTimeout:   proxy.Config.Timing.ReadTimeout,
		ExpectContinueTimeout: proxy.Config.Timing.ReadTimeout,
	}

	proxy.responderConfig = proxy.Config
	if proxy.DownstreamSegmentLength > 0 {
		proxy.responderConfig.MaxSegmentLenExclHeader = proxy.DownstreamSegmentLength
	}

	var err error
	if proxy.DNSResolver == "" {
		proxy.dnsConfig, err = dns.ClientConfigFromFile("/etc/resolv.conf")
		if err != nil {
			return err
		}
		if len(proxy.dnsConfig.Servers) == 0 {
			return fmt.Errorf("resolv.conf appears to be malformed or empty, try specifying an explicit DNS resolver address instead.")
		}
	} else {
		host, port, err := net.SplitHostPort(proxy.DNSResolver)
		if err != nil {
			return fmt.Errorf("failed to parse ip:port from DNS resolver %q", err)
		}
		portInt, err := strconv.Atoi(port)
		if err != nil {
			return fmt.Errorf("failed to parse ip:port from DNS resolver %q", err)
		}
		proxy.dnsConfig = &dns.ClientConfig{
			Servers: []string{host},
			Port:    strconv.Itoa(portInt),
		}
	}
	return nil
}

// dialContet returns a network connection tunnelled by the TCP-over-DNS proxy.
func (proxy *HTTPProxyServer) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	_, curr, _, err := toolbox.GetTwoFACodes(proxy.RequestOTPSecret)
	if err != nil {
		return nil, err
	}
	initiatorSegment, err := json.Marshal(ProxyRequest{
		Network:    network,
		Address:    addr,
		AccessTOTP: curr,
	})
	if err != nil {
		return nil, err
	}
	tcID := uint16(rand.Int())
	proxyServerIn, inTransport := net.Pipe()
	// Construct a client-side transmission control.
	proxy.logger.Info(fmt.Sprint(tcID), nil, "creating transmission control for %s using remote config: %+v", string(initiatorSegment), proxy.responderConfig)
	tc := &tcpoverdns.TransmissionControl{
		LogTag:               "HTTPProxyServer",
		ID:                   tcID,
		Debug:                proxy.Debug,
		InitiatorSegmentData: initiatorSegment,
		// The config for remote may differ by having a longer segment length.
		InitiatorConfig: proxy.responderConfig,
		Initiator:       true,
		InputTransport:  inTransport,
		MaxLifetime:     MaxProxyConnectionLifetime,
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
	proxy.Config.Config(tc)
	conn := &ProxiedConnection{
		dnsHostName:       proxy.DNSHostName,
		dnsConfig:         proxy.dnsConfig,
		dropPercentage:    proxy.dropPercentage,
		debug:             proxy.Debug,
		enableTXTRequests: proxy.EnableTXTRequests,
		in:                proxyServerIn,
		tc:                tc,
		context:           ctx,
		logger: &lalog.Logger{
			ComponentName: "HTTPProxyServerConn",
			ComponentID: []lalog.LoggerIDField{
				{Key: "TCID", Value: tc.ID},
			},
		},
	}
	// Start returns after the local transmission control transitions to the
	// established state.
	return conn.tc, conn.Start()
}

// ProxyHandler is an HTTP handler function that uses TCP-over-DNS proxy to
// transport requests and responses.
func (proxy *HTTPProxyServer) ProxyHandler(w http.ResponseWriter, r *http.Request) {
	clientIP := middleware.GetRealClientIP(r)
	switch r.Method {
	case http.MethodConnect:
		// Connect to the destination over TCP-over-DNS.
		dstConn, err := proxy.dialContext(r.Context(), "tcp", r.Host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		// OK to CONNECT.
		w.WriteHeader(http.StatusOK)
		// Tap into the data stream to transport data back and forth.
		hijackedStream, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "", http.StatusInternalServerError)
			proxy.logger.Warning(clientIP, nil, "connection stream cannot be tapped into")
			return
		}
		reqConn, _, err := hijackedStream.Hijack()
		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			proxy.logger.Warning(clientIP, err, "failed to tap into HTTP connection stream")
			return
		}
		go func() {
			_, _ = io.Copy(reqConn, dstConn)
			_ = reqConn.Close()
			_ = dstConn.Close()
		}()
		_, _ = io.Copy(dstConn, reqConn)
		_ = reqConn.Close()
		_ = dstConn.Close()
	default:
		// Execute the request as-is without handling higher-level mechanisms such as cookies and redirects
		resp, err := proxy.httpTransport.RoundTrip(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		// Copy all received headers to the client
		for key, vals := range resp.Header {
			for _, val := range vals {
				w.Header().Add(key, val)
			}
		}
		// Copy status code and response body to the client
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			proxy.logger.Warning(clientIP, err, "failed to copy response body back to client")
		}
	}
}

// StartAndBlock starts a web server to serve the HTTP(S) proxy endpoint.
// The function will block caller until Stop is called.
func (proxy *HTTPProxyServer) StartAndBlock() error {
	proxy.httpServer = &http.Server{
		Addr:         net.JoinHostPort(proxy.Address, strconv.Itoa(proxy.Port)),
		Handler:      proxy.proxyHandlerWithMiddleware,
		ReadTimeout:  proxy.Config.Timing.ReadTimeout,
		WriteTimeout: proxy.Config.Timing.WriteTimeout,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	proxy.logger.Info("", nil, "starting now")
	if err := proxy.httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("httpproxy.StartAndBlock.: failed to listen on %s:%d - %v", proxy.Address, proxy.Port, err)
	}
	return nil
}

// Stop the proxy server.
func (proxy *HTTPProxyServer) Stop() {
	proxy.cancelFun()
	if proxy.httpServer != nil {
		stopCtx, cancelFunc := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelFunc()
		if err := proxy.httpServer.Shutdown(stopCtx); err != nil {
			proxy.logger.Warning(proxy.Address, err, "failed to shutdown")
		}
	}
}
