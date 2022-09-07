package dnsclient

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/miekg/dns"
)

// ProxiedConnection handles an individual proxy connection to transport
// data between local transmission control and the one on the remote DNS proxy
// server.
type ProxiedConnection struct {
	client  *Client
	in      net.Conn
	tc      *tcpoverdns.TransmissionControl
	buf     *tcpoverdns.SegmentBuffer
	context context.Context
	logger  lalog.Logger
}

// Start configures and then starts the transmission control on local side, and
// spawns a background goroutine to transport segments back and forth using
// DNS queries.
// The function returns when the local transmission control transitions to the
// established state, or an error.
func (conn *ProxiedConnection) Start() error {
	conn.logger.Info("Start", "", nil, "start transporting data over DNS")
	conn.buf = tcpoverdns.NewSegmentBuffer(conn.logger, conn.tc.Debug, conn.tc.MaxSegmentLenExclHeader)
	// Absorb outgoing segments into the outgoing backlog.
	conn.tc.OutputSegmentCallback = conn.buf.Absorb
	conn.tc.Start(conn.context)
	// Start transporting segments back and forth.
	go conn.transportLoop()
	if !conn.tc.WaitState(conn.context, tcpoverdns.StateEstablished) {
		return fmt.Errorf("local transmission control failed to complete handshake")
	}
	return nil
}

func (conn *ProxiedConnection) lookupCNAME(queryName string) (string, error) {
	if len(queryName) < 3 {
		return "", errors.New("the input query name is too short")
	}
	if queryName[len(queryName)-1] != '.' {
		queryName += "."
	}
	client := new(dns.Client)
	query := new(dns.Msg)
	query.RecursionDesired = true
	query.SetQuestion(queryName, dns.TypeA)
	query.SetEdns0(dnsd.EDNSBufferSize, false)
	response, _, err := client.Exchange(query, fmt.Sprintf("%s:%s", conn.client.dnsConfig.Servers[0], conn.client.dnsConfig.Port))
	if err != nil {
		return "", err
	}
	if len(response.Answer) == 0 {
		return "", errors.New("the DNS query did not receive a response")
	}
	if cname, ok := response.Answer[0].(*dns.CNAME); ok {
		if rand.Intn(100) < conn.client.dropPercentage {
			return "", errors.New("dropped for testing")
		}
		return cname.Target, nil
	} else {
		return "", fmt.Errorf("the response answer %v is not a CNAME", response.Answer[0])
	}
}

func (conn *ProxiedConnection) transportLoop() {
	defer func() {
		// Linger briefly, then send the last segment. The brief waiting
		// time allows the TC to transition to the closed state.
		time.Sleep(5 * time.Second)
		final, exists := conn.buf.Latest()
		if exists && final.Flags != 0 {
			if _, err := conn.lookupCNAME(final.DNSName(fmt.Sprintf("%c", dnsd.ProxyPrefix), conn.client.DNSHostName)); err != nil {
				conn.logger.Warning("Start", "", err, "failed to send the final segment")
			}
		}
		conn.logger.Info("Start", "", nil, "DNS data transport finished, the final segment was: %v", final)
	}()
	countHostNameLabels := dnsd.CountNameLabels(conn.client.DNSHostName)
	for {
		if conn.tc.State() == tcpoverdns.StateClosed {
			return
		}
		var incomingSeg, outgoingSeg, nextInBacklog tcpoverdns.Segment
		var exists bool
		var cname string
		var err error
		begin := time.Now()
		// Pop a segment.
		outgoingSeg, exists = conn.buf.Pop()
		if !exists {
			// Wait for a segment.
			goto busyWaitInterval
		}
		// Turn the segment into a DNS query and send the query out
		// (data.data.data.example.com).
		cname, err = conn.lookupCNAME(outgoingSeg.DNSName(fmt.Sprintf("%c", dnsd.ProxyPrefix), conn.client.DNSHostName))
		conn.client.logger.Info("Start", fmt.Sprint(conn.tc.ID), nil, "sent over DNS query in %dms: %+v", time.Since(begin).Milliseconds(), outgoingSeg)
		if err != nil {
			conn.client.logger.Warning("Start", fmt.Sprint(conn.tc.ID), err, "failed to send output segment %v", outgoingSeg)
			conn.tc.IncreaseTimingInterval()
			goto busyWaitInterval
		}
		// Decode a segment from DNS query response and give it to the local
		// TC.
		incomingSeg = tcpoverdns.SegmentFromDNSName(countHostNameLabels, cname)
		if conn.client.Debug {
			conn.client.logger.Info("Start", fmt.Sprint(conn.tc.ID), nil, "DNS query response segment: %v", incomingSeg)
		}
		if !incomingSeg.Flags.Has(tcpoverdns.FlagMalformed) {
			if incomingSeg.Flags.Has(tcpoverdns.FlagKeepAlive) {
				// Increase the timing interval interval with each input
				// segment that does not carry data.
				conn.tc.IncreaseTimingInterval()
			} else {
				// Decrease the timing interval with each input segment that
				// carries data. This helps to temporarily increase the
				// throughput.
				conn.tc.DecreaseTimingInterval()
			}
			if _, err := conn.in.Write(incomingSeg.Packet()); err != nil {
				conn.client.logger.Warning("Start", fmt.Sprint(conn.tc.ID), err, "failed to receive input segment %v", incomingSeg)
				conn.tc.IncreaseTimingInterval()
				goto busyWaitInterval
			}
		}
		// If the next output segment carries useful data or flags, then
		// send it out without delay.
		nextInBacklog, exists = conn.buf.First()
		if exists && len(nextInBacklog.Data) > 0 || nextInBacklog.Flags != 0 {
			continue
		}
		// If the input segment carried useful data, then shorten the
		// waiting interval. Transmission control should be sending out an
		// acknowledgement fairly soon.
		if len(incomingSeg.Data) > 0 && !incomingSeg.Flags.Has(tcpoverdns.FlagKeepAlive) {
			select {
			case <-time.After(time.Duration(conn.tc.LiveTimingInterval().AckDelay * 8 / 7)):
				continue
			case <-conn.context.Done():
				return
			}
		}
		// Wait slightly longer than the keep-alive interval.
		select {
		case <-time.After(time.Duration(conn.tc.LiveTimingInterval().KeepAliveInterval * 8 / 7)):
			continue
		case <-conn.context.Done():
			return
		}
	busyWaitInterval:
		select {
		case <-time.After(tcpoverdns.BusyWaitInterval):
			continue
		case <-conn.context.Done():
			return
		}
	}
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
	// RequestOTPSecret is a TOTP secret for authorising outgoing connection
	// requests.
	RequestOTPSecret string `json:"RequestOTPSecret"`

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

	dnsConfig *dns.ClientConfig
	// dropPercentage is the percentage of resposnes to be dropped (returned as
	// error). This is for internal testing only.
	dropPercentage             int
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
		IdleConnTimeout:       client.Config.Timing.ReadTimeout,
		TLSHandshakeTimeout:   client.Config.Timing.ReadTimeout,
		ExpectContinueTimeout: client.Config.Timing.ReadTimeout,
	}

	var err error
	if client.DNSResolverAddr == "" {
		client.dnsConfig, err = dns.ClientConfigFromFile("/etc/resolv.conf")
		if err != nil {
			return err
		}
		if len(client.dnsConfig.Servers) == 0 {
			return fmt.Errorf("client.Initialise: resolv.conf appears to be malformed or empty, try specifying an explicit DNS resolver address instead.")
		}
	} else {
		client.dnsConfig = &dns.ClientConfig{
			Servers: []string{client.DNSResolverAddr},
			Port:    strconv.Itoa(client.DNSResovlerPort),
		}
	}
	return nil
}

// dialContet returns a network connection tunnelled by the TCP-over-DNS proxy.
func (client *Client) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	_, curr, _, err := toolbox.GetTwoFACodes(client.RequestOTPSecret)
	if err != nil {
		return nil, err
	}
	initiatorSegment, err := json.Marshal(dnsd.ProxyRequest{
		Network:    network,
		Address:    addr,
		AccessTOTP: curr,
	})
	if err != nil {
		return nil, err
	}
	tcID := uint16(rand.Int())
	clientIn, inTransport := net.Pipe()
	// Construct a client-side transmission control.
	client.logger.Info("dialContext", fmt.Sprint(tcID), nil, "creating transmission control for %s", string(initiatorSegment))
	tc := &tcpoverdns.TransmissionControl{
		LogTag:               "ProxyClient",
		ID:                   uint16(rand.Int()),
		Debug:                client.Debug,
		InitiatorSegmentData: initiatorSegment,
		InitiatorConfig:      client.Config,
		Initiator:            true,
		InputTransport:       inTransport,
		// In practice there are occasionally bursts of tens of errors at a
		// time before recovery.
		MaxTransportErrors: 300,
		// The duration of all retransmissions (if all go unacknowledged) is
		// MaxRetransmissions x SlidingWindowWaitDuration.
		MaxRetransmissions: 300,
		// The output transport is not used. Instead, the output segments
		// are kept in a backlog.
		OutputTransport: ioutil.Discard,
	}
	client.Config.Config(tc)
	conn := &ProxiedConnection{
		client:  client,
		in:      clientIn,
		tc:      tc,
		context: ctx,
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
		// Keep the buffer to minimum to improve responsiveness.
		// The buffer size has nothing to do with segment size.
		go misc.PipeConn(client.logger, true, client.Config.Timing.ReadTimeout, 1, dstConn, reqConn)
		misc.PipeConn(client.logger, true, client.Config.Timing.ReadTimeout, 1, reqConn, dstConn)
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
		ReadTimeout:  client.Config.Timing.ReadTimeout,
		WriteTimeout: client.Config.Timing.WriteTimeout,
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

// OptimalSegLen returns the optimal segment length appropriate for the DNS host
// name.
func OptimalSegLen(dnsHostName string) int {
	// The maximum DNS host name is 253 characters.
	// At present the encoding efficiency is ~62% at the worst case scenario.
	approxLen := float64(250-len(dnsHostName)) * 0.62
	ret := int(approxLen)
	if ret < 0 {
		return 0
	}
	return ret
}
