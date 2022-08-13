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
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/daemon/httpd/middleware"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
)

// ProxiedConnection handles an individual proxy connection to transport
// data between local transmission control and the one on the remote DNS proxy
// server.
type ProxiedConnection struct {
	client               *Client
	out, in              net.Conn
	tc                   *tcpoverdns.TransmissionControl
	outputSegmentBacklog []tcpoverdns.Segment
	mutex                *sync.Mutex
	context              context.Context
	logger               lalog.Logger
}

// Start configures and then starts the transmission control on local side, and
// spawns a background goroutine to transport segments back and forth using
// DNS queries.
// The function returns when the local transmission control transitions to the
// established state, or an error.
func (conn *ProxiedConnection) Start() error {
	if conn.client.Debug {
		conn.logger.Info("ProxyConnection.Start", "", nil, "starting now")
	}
	// Absorb outgoing segments into the outgoing backlog.
	conn.tc.OutputSegmentCallback = func(seg tcpoverdns.Segment) {
		// Replace the latest keep-alive or ack-only segment (if any), and
		// de-duplicate adjacent identical segments. These measures not only
		// speed up the exchanges but also ensure that peers can communicate
		// properly even if their timing characteristics differ.
		conn.mutex.Lock()
		defer conn.mutex.Unlock()
		var latest tcpoverdns.Segment
		if len(conn.outputSegmentBacklog) > 0 {
			latest = conn.outputSegmentBacklog[len(conn.outputSegmentBacklog)-1]
		}
		if latest.Flags.Has(tcpoverdns.FlagAckOnly) || latest.Flags.Has(tcpoverdns.FlagKeepAlive) {
			// Substitute the ack-only or keep-alive segment with the latest.
			if conn.client.Debug {
				conn.logger.Info("Start", "", nil, "callback is removing ack/keepalive segment: %+v", seg)
			}
			conn.outputSegmentBacklog[len(conn.outputSegmentBacklog)-1] = seg
		} else if latest.Equals(seg) {
			// De-duplicate adjacent identical segments.
			if conn.client.Debug {
				conn.logger.Info("Start", "", nil, "callback is removing duplicated segment: %+v", seg)
			}
		} else {
			conn.logger.Info("Start", "", nil, "callback is handling segment %+v", seg)
			conn.outputSegmentBacklog = append(conn.outputSegmentBacklog, seg)
		}
	}
	conn.tc.Start(conn.context)
	// Start transporting segments back and forth.
	go func() {
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return net.Dial(network, fmt.Sprintf("%s:%d", conn.client.DNSResolverAddr, conn.client.DNSResovlerPort))
			},
		}
		for {
			// Pop a segment.
			conn.mutex.Lock()
			if len(conn.outputSegmentBacklog) == 0 {
				conn.mutex.Unlock()
				continue
			}
			outgoingSeg := conn.outputSegmentBacklog[0]
			conn.outputSegmentBacklog = conn.outputSegmentBacklog[1:]
			conn.mutex.Unlock()
			// Turn the segment into a DNS query and send the query out
			// (data.data.data.example.com).
			if conn.client.Debug {
				conn.client.logger.Info("pipeSegments", fmt.Sprint(conn.tc.ID), nil, "sending output segment over DNS query: %v", outgoingSeg)
			}
			addrs, err := resolver.LookupIP(conn.context, "ip4", outgoingSeg.DNSNameQuery(fmt.Sprintf("%c", dnsd.ProxyPrefix), conn.client.DNSHostName))
			if err != nil {
				conn.client.logger.Warning("pipeSegments", fmt.Sprint(conn.tc.ID), err, "failed to send output segment %v", outgoingSeg)
				continue
			}
			// Decode a segment from DNS query response and give it to the local TC.
			incomingSeg := tcpoverdns.SegmentFromIPs(addrs)
			if conn.client.Debug {
				conn.client.logger.Info("pipeSegments", fmt.Sprint(conn.tc.ID), nil, "DNS query response segment: %v", incomingSeg)
			}
			if !incomingSeg.Flags.Has(tcpoverdns.FlagMalformed) {
				if _, err := conn.in.Write(incomingSeg.Packet()); err != nil {
					conn.client.logger.Warning("pipeSegments", fmt.Sprint(conn.tc.ID), err, "failed to receive input segment %v", incomingSeg)
					continue
				}
			}
			// If there are more segments waiting to be sent, then send the next one
			// right away without waiting for the keep-alive interval.
			conn.mutex.Lock()
			lenRemaining := len(conn.outputSegmentBacklog)
			conn.mutex.Unlock()
			if lenRemaining > 0 {
				continue
			}
			// Wait for keep-alive interval.
			select {
			case <-time.After(time.Duration(conn.client.Config.KeepAliveIntervalSec) * time.Second * 80 / 100):
			case <-conn.context.Done():
				return
			}
		}
	}()
	if !conn.tc.WaitState(conn.context, tcpoverdns.StateEstablished) {
		return fmt.Errorf("local transmission control failed to complete handshake")
	}
	return nil
}

// Client is an HTTP proxy server that tunnels its HTTP clients' traffic through
// TCP-over-DNS proxy.
type Client struct {
	// Address is the network address for the HTTP proxy to listen on, e.g.
	// 127.0.0.1 to serve the localhost alone.
	Address string `json:"Address"`
	// Port to listen on.
	Port int `json:"Port"`
	// Config contains the parameters for the initiator of the proxy
	// connections to configure the remote transmission control.
	Config tcpoverdns.InitiatorConfig
	// Debug enables verbose logging for IO activities.
	Debug bool

	// httpTransport is the HTTP round tripper used by the proxy handler for
	// HTTP (unencrypted) proxy requests. This transport is not used for handling
	// HTTPS (HTTP CONNECT) requests.
	httpTransport *http.Transport

	// DNSResolverAddr is the address of the (public) recursive DNS resolver.
	DNSResolverAddr string
	// DNSResovlerPort is the port number of the (public) recursive DNS resolver.
	DNSResovlerPort int
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
	client.proxyHandlerWithMiddleware = middleware.LogRequestStats(client.logger, middleware.EmergencyLockdown(client.ProxyHandler))
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

// dialContet returns a network connection tunnelled by the TCP-over-DNS proxy.
func (client *Client) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	initiatorSegment, err := json.Marshal(dnsd.ProxyRequest{Network: network, Address: addr})
	if err != nil {
		return nil, err
	}
	tcID := uint16(rand.Int())
	clientIn, inTransport := net.Pipe()
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
		// The output transport is not used. Instead, the output segments
		// are kept in a backlog.
		OutputTransport: io.Discard,
	}
	client.Config.Config(tc)
	conn := &ProxiedConnection{
		client:               client,
		in:                   clientIn,
		tc:                   tc,
		context:              client.context,
		outputSegmentBacklog: make([]tcpoverdns.Segment, 0),
		mutex:                new(sync.Mutex),
		logger: lalog.Logger{
			ComponentName: "DNSClientProxyConn",
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
func (client *Client) ProxyHandler(w http.ResponseWriter, r *http.Request) {
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