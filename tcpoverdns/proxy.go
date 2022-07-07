package tcpoverdns

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
)

type ProxyRequest struct {
	Port    int    `json:"p"`
	Address string `json:"a"`
}

// ProxyConnection consists of a transmission control paired to a TCP connection
// relayed by the transmission control.
type ProxyConnection struct {
	proxy                *Proxy
	tcpConn              *net.TCPConn
	context              context.Context
	tc                   *TransmissionControl
	inputSegments        net.Conn
	outputSegmentBacklog []Segment
	mutex                *sync.Mutex
}

// Start piping data back and forth between proxy TCP connection and
// transmission control.
func (conn *ProxyConnection) Start() {
	if conn.proxy.Debug {
		conn.proxy.Logger.Info("ProxyConnection.Start", "", nil, "starting now")
	}
	defer func() {
		_ = conn.Close()
		conn.proxy.mutex.Lock()
		delete(conn.proxy.connections, conn.tc.ID)
		conn.proxy.mutex.Unlock()

	}()
	// Absorb outgoing segments into the outgoing backlog.
	conn.tc.OutputSegmentCallback = func(seg Segment) {
		conn.outputSegmentBacklog = append(conn.outputSegmentBacklog, seg)
	}
	// Carry on with the handshake.
	conn.tc.Start(conn.context)
	for {
		if conn.tc.WaitState(conn.context, StateEstablished) {
			break
		}
	}
	if conn.proxy.Debug {
		conn.proxy.Logger.Info("ProxyConnection.Start", "", nil, "TC is established")
	}
	// Pipe data in both directions.
	go func() {
		// This goroutine automatically terminates when Pipe encounters an IO
		// error.
		if err := misc.Pipe(conn.proxy.ReadBufferSize, conn.tc, conn.tcpConn); err != nil {
			if conn.proxy.Debug {
				conn.proxy.Logger.Info("ProxyConnection.Start", "", err, "finished piping from TC to TCP connection")
			}

		}
	}()
	if err := misc.Pipe(conn.proxy.ReadBufferSize, conn.tcpConn, conn.tc); err != nil {
		if conn.proxy.Debug {
			conn.proxy.Logger.Info("ProxyConnection.Start", "", err, "finished piping from TCP connection to TC")
		}
		return
	}
}

// WaitSegment busy-waits until a new segment is available from the output
// segment backlog, and then pops the segment.
func (conn *ProxyConnection) WaitSegment(ctx context.Context) (Segment, bool) {
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
				return Segment{}, false
			case <-time.After(BusyWaitInterval):
				continue
			}
		}
	}
}

// Close and terminate the proxy TCP connection and its transmission control.
func (conn *ProxyConnection) Close() error {
	conn.tc.Close()
	_ = conn.tcpConn.Close()
	return nil
}

// Proxy manages the full life cycle of multiple transmission controls created
// for the purpose of relaying TCP connections.
type Proxy struct {
	// ReadBufferSize is the buffer
	ReadBufferSize int
	// MaxLifetime is the maximum duration of a proxy TCP connection as well as
	// its transmission control.
	MaxLifetime time.Duration

	// MaxReplyLatency is the maximum duration to wait for a reply (outgoing)
	// segment before returning from the Receive function.
	MaxReplyDelay time.Duration

	// DialTimeout is the timeout used for creating new a proxy TCP connection.
	DialTimeout time.Duration

	// Debug enables verbose logging for IO activities.
	Debug bool
	// Logger is used to log IO activities when verbose logging is enabled.
	Logger lalog.Logger

	connections map[uint16]*ProxyConnection
	context     context.Context
	cancelFun   func()
	mutex       *sync.Mutex
}

// Start initialises the internal state of the proxy.
func (proxy *Proxy) Start(ctx context.Context) {
	if proxy.ReadBufferSize == 0 {
		// Keep the buffer size small.
		proxy.ReadBufferSize = 256
	}
	if proxy.MaxLifetime == 0 {
		proxy.MaxLifetime = 10 * time.Minute
	}
	if proxy.MaxReplyDelay == 0 {
		proxy.MaxReplyDelay = 3 * time.Second
	}
	if proxy.DialTimeout == 0 {
		proxy.DialTimeout = 10 * time.Second
	}
	proxy.connections = make(map[uint16]*ProxyConnection)
	proxy.context, proxy.cancelFun = context.WithCancel(ctx)
	proxy.mutex = new(sync.Mutex)
	proxy.Logger = lalog.Logger{ComponentName: "TCProxy"}
}

// Receive processes an incoming segment and relay the segment to an existing
// transmission control, or create a new transmission control for the proxy
// destination.
func (proxy *Proxy) Receive(in Segment) (Segment, bool) {
	proxy.mutex.Lock()
	conn, exists := proxy.connections[in.ID]
	proxy.mutex.Unlock()
	if exists {
		// Pass the segment to transmission control's input transport.
		if proxy.Debug {
			proxy.Logger.Info("Receive", "", nil, "hand over segment %+v to a known TC", in)
		}
		if _, err := conn.inputSegments.Write(in.Packet()); err != nil {
			conn.Close()
		}
	} else {
		// Connect to the proxy destination.
		var req ProxyRequest
		if err := json.Unmarshal(in.Data, &req); err != nil {
			proxy.Logger.Warning("Receive", "", err, "failed to deserialise proxy request")
			return Segment{}, false
		}
		if proxy.Debug {
			proxy.Logger.Info("Receive", "", nil, "new connection request - seg: %+v, req: %+v", in, req)
		}
		dest := fmt.Sprintf("%s:%d", req.Address, req.Port)
		netConn, err := net.DialTimeout("tcp", dest, proxy.DialTimeout)
		if err != nil {
			proxy.Logger.Warning("Receive", "", err, "failed to connect to proxy destination %s", dest)
		}
		tcpConn := netConn.(*net.TCPConn)
		misc.TweakTCPConnection(tcpConn, proxy.MaxLifetime)
		// Establish the transmission control by completing the handshake.
		proxyIn, tcIn := net.Pipe()
		tc := &TransmissionControl{
			// This transmission control is a responder during the handshake.
			Initiator:      false,
			InputTransport: tcIn,
			MaxLifetime:    proxy.MaxLifetime,
			// The output transport is not used. Instead, the output segments
			// are kept in a backlog.
			OutputTransport: io.Discard,
			ID:              9999, // TODO FIXME: change this to input TC ID.
		}
		// Track the new connection.
		conn = &ProxyConnection{
			proxy:         proxy,
			tcpConn:       tcpConn,
			context:       proxy.context,
			tc:            tc,
			inputSegments: proxyIn,
			mutex:         new(sync.Mutex),
		}
		proxy.mutex.Lock()
		proxy.connections[in.ID] = conn
		proxy.mutex.Unlock()
		go conn.Start()
	}
	if proxy.Debug {
		proxy.Logger.Info("Receive", "", nil, "waiting for a reply outbound segment")
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
