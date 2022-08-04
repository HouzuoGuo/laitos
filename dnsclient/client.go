package dnsclient

import (
	"context"
	"encoding/json"
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
	client.logger = lalog.Logger{ComponentName: "tcpoverdns-client", ComponentID: []lalog.LoggerIDField{{Key: "Port", Value: strconv.Itoa(client.Port)}}}
	client.proxyHandlerWithMiddleware = middleware.LogRequestStats(client.logger,
		middleware.RecordInternalStats(misc.TCPOverDNSClientStats,
			middleware.EmergencyLockdown(client.ProxyHandler)))
	client.context, client.cancelFun = context.WithCancel(ctx)

	client.httpTransport = &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   120 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          100,
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   120 * time.Second,
		ExpectContinueTimeout: 120 * time.Second,
	}
	return nil
}

func (client *Client) pipeSegments(out, in net.Conn, tc *tcpoverdns.TransmissionControl, dnsServerAddr string) {
	_ = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (conn net.Conn, e error) {
			var d net.Dialer
			return d.DialContext(ctx, network, dnsServerAddr)
		},
	}
	for {
		// Pipe segments from TC to proxy.
		seg := tcpoverdns.ReadSegment(context.Background(), out)
		if client.Debug {
			lalog.DefaultLogger.Info("pipeSegments", fmt.Sprint(tc.ID), nil, "sending output segment %v over a DNS query", seg)
		}

		// Send the output over DNS to the proxy destination TC.
		// TODO FIXME: use a DNS client to send out the outbound segment in a query, and interpret the query response as an inbound segment.
		lalog.DefaultLogger.Info("", "", nil, "proxy tc replies to test: %+v, %v", resp, hasResp)
		if hasResp {
			// Send the response segment back to TC.
			_, err := testIn.Write(resp.Packet())
			if err != nil {
				panic("failed to write to testIn")
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
	// out.read -> DNS query (data.data.example.com)
	// DNS query response -> in.write
	client.logger.Info("dialContext", fmt.Sprint(tcID), nil, "dialing %s", string(initiatorSegment))
	tc := &tcpoverdns.TransmissionControl{
		LogTag:               "dialContext",
		ID:                   uint16(rand.Int()),
		InitiatorSegmentData: []byte(`{"p": 443, "a": "203.0.113.0"}`),
		Initiator:            true,
		InputTransport:       inTransport,
		OutputTransport:      outTransport,
	}
	client.Config.Config(tc)
	tc.Start(client.context)
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
		go misc.PipeConn(client.logger, true, time.Duration(client.Config.IOTimeoutSec)*time.Second, 1280, dstConn, reqConn)
		misc.PipeConn(client.logger, true, time.Duration(client.Config.IOTimeoutSec)*time.Second, 1280, reqConn, dstConn)
	default:
		// Execute the request as-is without handling higher-level mechanisms such as cookies and redirects
		resp, err := httpTransport.RoundTrip(r)
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
