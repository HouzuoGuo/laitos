package dnsd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
	"github.com/HouzuoGuo/laitos/toolbox"
)

// ProxyRequest is the data sent by a proxy client to initiate a connection
// toward a proxy destination.
type ProxyRequest struct {
	// Network name of the address (e.g. "tcp"), this takes precedence over the
	// port number when the network name is specified.
	Network string `json:"n"`
	// Port number in the absence of network name.
	Port int `json:"p"`
	// Address is the host IP address or network address (IP:port).
	Address string `json:"a"`
	// AccessTOTP is a time-based OTP that authorises the connection request.
	AccessTOTP string `json:"t"`
}

// ProxyConnection consists of a transmission control paired to a TCP connection
// relayed by the transmission control.
type ProxyConnection struct {
	proxy                *Proxy
	tcpConn              *net.TCPConn
	context              context.Context
	tc                   *tcpoverdns.TransmissionControl
	inputSegments        net.Conn
	outputSegmentBacklog []tcpoverdns.Segment
	mutex                *sync.Mutex
	logger               lalog.Logger
}

// Start piping data back and forth between proxy TCP connection and
// transmission control.
// The function blocks until the underlying TC is closed.
func (conn *ProxyConnection) Start() {
	if conn.proxy.Debug {
		conn.logger.Info("Start", "", nil, "starting now")
	}
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		if conn.proxy.Debug {
			conn.logger.Info("Start", "", nil, "closing and lingering")
			conn.tc.DumpState()
		}
		_ = conn.Close()
		misc.TCPOverDNSStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
		go func() {
			// Linger a short while before deleting (cease tracking) the
			// connection, as the output segment buffer may still contain
			// useful, final few segments - and crucially it contains the
			// segment with ResetTerminate flag which is expected by the peer
			// transmission control.
			time.Sleep(conn.proxy.Linger)
			conn.proxy.mutex.Lock()
			delete(conn.proxy.connections, conn.tc.ID)
			conn.proxy.mutex.Unlock()
			if conn.proxy.Debug {
				conn.logger.Info("Start", "", nil, "closed and removed from proxy")
			}
		}()
	}()
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
			// These segments may contain random data that are not useful.
			if conn.proxy.Debug {
				conn.logger.Info("Start", "", nil, "callback is removing duplicated ack/keepalive segment: %+v", conn.outputSegmentBacklog[len(conn.outputSegmentBacklog)-1])
			}
			conn.outputSegmentBacklog[len(conn.outputSegmentBacklog)-1] = seg
		} else if latest.Equals(seg) {
			// De-duplicate adjacent identical segments.
			if conn.proxy.Debug {
				conn.logger.Info("Start", "", nil, "callback is removing duplicated segment: %+v", seg)
			}
		} else {
			conn.logger.Info("Start", "", nil, "callback is handling segment %+v", seg)
			conn.outputSegmentBacklog = append(conn.outputSegmentBacklog, seg)
		}
	}
	// Carry on with the handshake.
	conn.tc.Start(conn.context)
	conn.tc.WaitState(conn.context, tcpoverdns.StateEstablished)
	if conn.proxy.Debug {
		conn.logger.Info("ProxyConnection.Start", "", nil, "TC is established")
	}
	// Pipe data in both directions.
	if conn.tcpConn != nil {
		// Keep the buffer to minimum to improve responsiveness.
		// The buffer size has nothing to do with segment size.
		go func() {
			if err := misc.PipeConn(conn.logger, false, conn.tc.MaxLifetime, 1, conn.tc, conn.tcpConn); err != nil {
				if conn.proxy.Debug {
					conn.logger.Info("ProxyConnection.Start", "", err, "finished piping from TC to TCP connection")
				}
			}
		}()
		if err := misc.PipeConn(conn.logger, false, conn.tc.MaxLifetime, 1, conn.tcpConn, conn.tc); err != nil {
			if conn.proxy.Debug {
				conn.logger.Info("ProxyConnection.Start", "", err, "finished piping from TCP connection to TC")
			}
		}
	}
	// Wait for the transmission control to close.
	conn.tc.CloseAfterDrained()
	conn.tc.WaitState(conn.context, tcpoverdns.StateClosed)
	// The proxy connection lingers for a short while, see defer.
}

// WaitSegment busy-waits until a new segment is available from the output
// segment backlog, and then pops the segment.
func (conn *ProxyConnection) WaitSegment(ctx context.Context) (tcpoverdns.Segment, bool) {
	for {
		conn.mutex.Lock()
		if len(conn.outputSegmentBacklog) > 0 {
			ret := conn.outputSegmentBacklog[0]
			conn.outputSegmentBacklog = conn.outputSegmentBacklog[1:]
			conn.mutex.Unlock()
			return ret, true
		} else {
			conn.mutex.Unlock()
			select {
			case <-ctx.Done():
				return tcpoverdns.Segment{}, false
			case <-time.After(tcpoverdns.BusyWaitInterval):
				continue
			}
		}
	}
}

// Close and terminate the proxy TCP connection and its transmission control.
func (conn *ProxyConnection) Close() error {
	if conn.tcpConn != nil {
		_ = conn.tcpConn.Close()
	}
	_ = conn.inputSegments.Close()
	_ = conn.tc.Close()
	return nil
}

// Proxy manages the full life cycle of multiple transmission controls created
// for the purpose of relaying TCP connections.
type Proxy struct {
	// DNSDaemon helps the proxy to identify advertising/malware proxy
	// destinations and to refuse serving their clients.
	DNSDaemon *Daemon `json:"-"`

	// MaxReplyLatency is the maximum duration to wait for a reply (outgoing)
	// segment before returning from the Receive function.
	MaxReplyDelay time.Duration `json:"-"`

	// RequestOTPSecret is a TOTP secret for authorising incoming connection
	// requests.
	RequestOTPSecret string `json:"RequestOTPSecret"`
	// Debug enables verbose logging for IO activities.
	Debug bool `json:"Debug"`

	// Linger is a brief period of time for a proxy connection to stay before it
	// is removed from the internal collection of proxy connections. The delay
	// is crucial to allow the final segments of each proxy connection to be
	// received by proxy client - including the segment with ResetTerminate.
	Linger time.Duration `json:"-"`

	// DialTimeout is the timeout used for creating new a proxy TCP connection.
	DialTimeout time.Duration `json:"-"`

	// logger is used to log IO activities when verbose logging is enabled.
	logger lalog.Logger `json:"-"`

	connections map[uint16]*ProxyConnection
	context     context.Context
	cancelFun   func()
	mutex       *sync.Mutex
}

// Start initialises the internal state of the proxy.
func (proxy *Proxy) Start(ctx context.Context) {
	if proxy.MaxReplyDelay == 0 {
		// This default should be greater/longer than transmission control's
		// default AckDelay, or the performance will suffer quite a bit.
		proxy.MaxReplyDelay = 2 * time.Second
	}
	if proxy.DialTimeout == 0 {
		proxy.DialTimeout = 10 * time.Second
	}
	if proxy.Linger == 0 {
		proxy.Linger = 60 * time.Second
	}
	proxy.connections = make(map[uint16]*ProxyConnection)
	proxy.context, proxy.cancelFun = context.WithCancel(ctx)
	proxy.mutex = new(sync.Mutex)
	proxy.logger = lalog.Logger{ComponentName: "TCProxy"}
}

// Receive processes an incoming segment and relay the segment to an existing
// transmission control, or create a new transmission control for the proxy
// destination.
func (proxy *Proxy) Receive(in tcpoverdns.Segment) (tcpoverdns.Segment, bool) {
	proxy.mutex.Lock()
	conn, exists := proxy.connections[in.ID]
	proxy.mutex.Unlock()
	if !exists {
		// Connect to the proxy destination.
		var req ProxyRequest
		proxy.logger.Info("Receive", in.ID, nil, "new connection request - seg: %+v, req: %+v", in, req)
		if len(in.Data) < tcpoverdns.InitiatorConfigLen {
			proxy.logger.Warning("Receive", in.ID, nil, "received a malformed segment possibly from a stale TC")
			return tcpoverdns.Segment{}, false
		}
		if err := json.Unmarshal(in.Data[tcpoverdns.InitiatorConfigLen:], &req); err != nil {
			proxy.logger.Warning("Receive", in.ID, err, "failed to deserialise proxy request")
			return tcpoverdns.Segment{}, false
		}
		if proxy.DNSDaemon != nil && proxy.DNSDaemon.IsInBlacklist(req.Address) {
			proxy.logger.Info("Receive", in.ID, nil, "refusing connection to blacklisted destination %q", req.Address)
			return tcpoverdns.Segment{ID: in.ID, Flags: tcpoverdns.FlagReset}, true
		}
		prev, curr, next, err := toolbox.GetTwoFACodes(proxy.RequestOTPSecret)
		if err != nil {
			proxy.logger.Info("Receive", in.ID, nil, "failed to calculate TOTP codes")
			return tcpoverdns.Segment{ID: in.ID, Flags: tcpoverdns.FlagReset}, true
		}
		if req.AccessTOTP != prev && req.AccessTOTP != curr && req.AccessTOTP != next {
			proxy.logger.Warning("Receive", in.ID, nil, "the request failed OTP check")
			return tcpoverdns.Segment{ID: in.ID, Flags: tcpoverdns.FlagReset}, true
		}
		// Construct the transmission control at proxy's side.
		proxyIn, tcIn := net.Pipe()
		tc := &tcpoverdns.TransmissionControl{
			Debug:  proxy.Debug,
			LogTag: "ProxyServer",
			ID:     in.ID,
			// This transmission control is a responder during the handshake.
			Initiator:      false,
			InputTransport: tcIn,
			MaxLifetime:    30 * time.Minute,
			// In practice there are occasionally bursts of tens of errors at a
			// time before recovery.
			MaxTransportErrors: 200,
			// The duration of all retransmissions (if all go unacknowledged) is
			// MaxRetransmissions x SlidingWindowWaitDuration.
			MaxRetransmissions: 200,
			// The output transport is not used. Instead, the output segments
			// are kept in a backlog.
			OutputTransport: io.Discard,
			// The segment length and sliding window length are set by the
			// initiator using InitiatorConfig.
		}
		// Connect to the intended destination.
		var dialNet, dialDest string
		if req.Network == "" {
			dialNet = "tcp"
			dialDest = fmt.Sprintf("%s:%d", req.Address, req.Port)
		} else {
			dialNet = req.Network
			dialDest = req.Address
		}
		netConn, err := net.DialTimeout(dialNet, dialDest, proxy.DialTimeout)
		if err != nil {
			// Immediately close the transmission control if the destination is
			// unreachable.
			proxy.logger.Warning("Receive", in.ID, err, "failed to connect to proxy destination %s %s", dialNet, dialDest)
			// Proceed with handshake, but there will be no data coming through
			// the transmission control and it will be closed shortly.
		}
		var tcpConn *net.TCPConn
		if netConn != nil {
			tcpConn = netConn.(*net.TCPConn)
			misc.TweakTCPConnection(tcpConn, tc.MaxLifetime)
		}
		// Track the new proxy connection.
		conn = &ProxyConnection{
			proxy:         proxy,
			tcpConn:       tcpConn,
			context:       proxy.context,
			tc:            tc,
			inputSegments: proxyIn,
			mutex:         new(sync.Mutex),
			logger: lalog.Logger{
				ComponentName: "ProxyServer",
				ComponentID: []lalog.LoggerIDField{
					{Key: "TCID", Value: tc.ID},
				},
			},
		}
		proxy.mutex.Lock()
		proxy.connections[in.ID] = conn
		proxy.mutex.Unlock()
		// Go ahead with handshake and data transfer.
		go conn.Start()
	}
	if _, err := conn.inputSegments.Write(in.Packet()); err != nil {
		_ = conn.Close()
	}
	if proxy.Debug {
		proxy.logger.Info("Receive", in.ID, nil, "waiting for a reply outbound segment")
	}
	waitCtx, cancel := context.WithTimeout(proxy.context, proxy.MaxReplyDelay)
	defer cancel()
	seg, hasSeg := conn.WaitSegment(waitCtx)
	return seg, hasSeg
}

// Close terminates all ongoing transmission controls.
// The function always returns nil.
func (proxy *Proxy) Close() error {
	proxy.mutex.Lock()
	// Terminate all TCs.
	for _, conn := range proxy.connections {
		_ = conn.Close()
	}
	defer proxy.mutex.Unlock()
	// Terminate all proxy connections.
	proxy.cancelFun()
	return nil
}
