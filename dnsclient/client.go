package dnsclient

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

	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/daemon/httpd/middleware"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
)

// Client is an HTTP proxy server that tunnels its HTTP clients' traffic through
// TCP-over-DNS proxy.
type Client struct {
	// Address is the network address for the HTTP proxy to listen on, e.g.
	// 127.0.0.1 to serve the localhost alone.
	Address string `json:"Address"`
	// Port to listen on.
	Port int `json:"Port"`
	// DNSDaemon is an initialised DNS daemon. The proxy server uses the daemon
	// to provide protection against advertising, malware, and tracking.
	DNSDaemon *dnsd.Daemon `json:"-"`
	// Config contains the parameters for the initiator of the proxy
	// connections to configure the remote transmission control.
	Config tcpoverdns.InitiatorConfig
	// Debug enables verbose logging for IO activities.
	Debug bool

	// httpTransport is the HTTP round tripper used by the proxy handler for
	// HTTP (unencrypted) proxy requests. This transport is not used for handling
	// HTTPS (HTTP CONNECT) requests.
	httpTransport *http.Transport

	// DNSServerAddr is the address of the (public) recursive DNS resolver.
	DNSServerAddr string
	// DNSServerPort is the port number of the (public) recursive DNS resolver.
	DNSServerPort int
	// DNSHostName is the host name of the TCP-over-DNS proxy server.
	DNSHostName string

	proxyHandlerWithMiddleware http.HandlerFunc
	logger                     lalog.Logger
	httpServer                 *http.Server
	context                    context.Context
	cancelFun                  func()
}

// Initialise validates configuration parameters and initialises the internal state of the daemon.
func (client *Client) Initialise(ctx context.Context) error {
	if client.Address == "" {
		client.Address = "127.0.0.1"
	}
	if client.Port == 0 {
		client.Port = 8080
	}
	if len(client.DNSHostName) < 3 {
		return fmt.Errorf("dnsclient: DNSDomainName (%q) must be a valid host name", client.DNSHostName)
	}
	if client.DNSHostName[0] == '.' {
		client.DNSHostName = client.DNSHostName[1:]
	}
	client.logger = lalog.Logger{ComponentName: "dnsclient", ComponentID: []lalog.LoggerIDField{{Key: "Port", Value: strconv.Itoa(client.Port)}}}
	client.proxyHandlerWithMiddleware = middleware.LogRequestStats(client.logger,
		middleware.RecordInternalStats(misc.TCPOverDNSClientStats,
			middleware.EmergencyLockdown(client.ProxyHandler)))
	client.context, client.cancelFun = context.WithCancel(ctx)

	client.httpTransport = &http.Transport{
		Proxy:                 nil,
		DialContext:           client.dialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          100,
		IdleConnTimeout:       time.Duration(client.Config.IOTimeoutSec) * time.Second,
		TLSHandshakeTimeout:   time.Duration(client.Config.IOTimeoutSec) * time.Second,
		ExpectContinueTimeout: time.Duration(client.Config.IOTimeoutSec) * time.Second,
	}
	return nil
}

func (client *Client) pipeSegments(out, in net.Conn, tc *tcpoverdns.TransmissionControl) {
	resolver := &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (conn net.Conn, e error) {
			fmt.Println("contacting", network, address)
			return net.Dial(network, fmt.Sprintf("%s:%d", client.DNSServerAddr, client.DNSServerPort))
		},
	}
	for {
		// Send the outgoing segments in DNS queries at a pace slightly faster
		// than the keep-alive interval.
		select {
		case <-time.After(time.Duration(client.Config.KeepAliveIntervalSec) * time.Second * 80 / 100):
		case <-client.context.Done():
			return
		}
		// out.Read -> DNS query (data.data.data.example.com).
		outgoingSeg := tcpoverdns.ReadSegment(context.Background(), out)
		if client.Debug {
			client.logger.Info("pipeSegments", fmt.Sprint(tc.ID), nil, "sending output segment over DNS query: %v", outgoingSeg)
		}
		addrs, err := resolver.LookupIP(client.context, "ip4", outgoingSeg.DNSNameQuery(fmt.Sprintf("%c", dnsd.ProxyPrefix), client.DNSHostName))
		if err != nil {
			client.logger.Warning("pipeSegments", fmt.Sprint(tc.ID), err, "failed to send output segment %v", outgoingSeg)
			continue
		}
		// DNS response -> in.Write.
		incomingSeg := tcpoverdns.SegmentFromIPs(addrs)
		if client.Debug {
			client.logger.Info("pipeSegments", fmt.Sprint(tc.ID), nil, "DNS query response segment: %v", incomingSeg)
		}
		if !incomingSeg.Flags.Has(tcpoverdns.FlagMalformed) {
			if _, err := in.Write(incomingSeg.Packet()); err != nil {
				client.logger.Warning("pipeSegments", fmt.Sprint(tc.ID), err, "failed to receive input segment %v", incomingSeg)
				continue
			}
		}
	}
}

// dialContet returns a network connection tunnelled by the TCP-over-DNS proxy.
func (client *Client) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	initiatorSegment, err := json.Marshal(tcpoverdns.ProxyRequest{Network: network, Address: addr})
	if err != nil {
		return nil, err
	}
	tcID := uint16(rand.Int())
	clientIn, inTransport := net.Pipe()
	clientOut, outTransport := net.Pipe()
	// Construct a client-side transmission control.
	client.logger.Info("dialContext", fmt.Sprint(tcID), nil, "creating transmission control for %s", string(initiatorSegment))
	tc := &tcpoverdns.TransmissionControl{
		LogTag:               "dialContext",
		ID:                   uint16(rand.Int()),
		Debug:                client.Debug,
		InitiatorSegmentData: initiatorSegment,
		InitiatorConfig:      client.Config,
		Initiator:            true,
		InputTransport:       inTransport,
		OutputTransport:      outTransport,
	}
	client.Config.Config(tc)
	tc.Start(client.context)
	// Start transporting data segments over DNS.
	go client.pipeSegments(clientOut, clientIn, tc)
	// TODO FIXME: use the same trick of proxy.go to de-duplicate identical output packets to improve the throughput.
	return tc, nil
}

// ProxyHandler is an HTTP handler function that uses TCP-over-DNS proxy to
// transport requests and responses.
func (client *Client) ProxyHandler(w http.ResponseWriter, r *http.Request) {
	// Pass the intended destination through DNS daemon's blacklist filter
	if client.DNSDaemon != nil && client.DNSDaemon.IsInBlacklist(r.Host) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	clientIP := middleware.GetRealClientIP(r)
	switch r.Method {
	case http.MethodConnect:
		// Connect to the destination over TCP-over-DNS.
		dstConn, err := client.dialContext(r.Context(), "tcp", r.Host)
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
			client.logger.Warning("ProxyHandler", clientIP, nil, "connection stream cannot be tapped into")
			return
		}
		reqConn, _, err := hijackedStream.Hijack()
		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			client.logger.Warning("ProxyHandler", clientIP, err, "failed to tap into HTTP connection stream")
			return
		}
		go misc.PipeConn(client.logger, true, time.Duration(client.Config.IOTimeoutSec)*time.Second, client.Config.MaxSegmentLenExclHeader, dstConn, reqConn)
		misc.PipeConn(client.logger, true, time.Duration(client.Config.IOTimeoutSec)*time.Second, client.Config.MaxSegmentLenExclHeader, reqConn, dstConn)
	default:
		// Execute the request as-is without handling higher-level mechanisms such as cookies and redirects
		resp, err := client.httpTransport.RoundTrip(r)
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
			client.logger.Warning("ProxyHandler", clientIP, err, "failed to copy response body back to client")
		}
	}
}

// StartAndBlock starts a web server to serve the HTTP(S) proxy endpoint.
// The function will block caller until Stop is called.
func (client *Client) StartAndBlock() error {
	client.httpServer = &http.Server{
		Addr:         net.JoinHostPort(client.Address, strconv.Itoa(client.Port)),
		Handler:      client.proxyHandlerWithMiddleware,
		ReadTimeout:  time.Duration(client.Config.IOTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(client.Config.IOTimeoutSec) * time.Second,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	client.logger.Info("StartAndBlock", "", nil, "starting now")
	if err := client.httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("httpproxy.StartAndBlock.: failed to listen on %s:%d - %v", client.Address, client.Port, err)
	}
	return nil
}

// Stop the client.
func (client *Client) Stop() {
	client.cancelFun()
	if client.httpServer != nil {
		stopCtx, cancelFunc := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelFunc()
		if err := client.httpServer.Shutdown(stopCtx); err != nil {
			client.logger.Warning("Stop", client.Address, err, "failed to shutdown")
		}
	}
}
