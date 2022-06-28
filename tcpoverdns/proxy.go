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

type proxyConnection struct {
	proxy                *Proxy
	tcpConn              *net.TCPConn
	context              context.Context
	tc                   *TransmissionControl
	inputSegments        net.Conn
	outputSegmentBacklog []Segment
}

// Start piping data back and forth between proxy TCP connection and
// transmission control.
func (conn *proxyConnection) Start() {
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
	// Pipe data in both directions.
	go func() {
		// This goroutine automatically terminates when Pipe encounters an IO
		// error.
		_ = misc.Pipe(conn.proxy.ReadBufferSize, conn.tc, conn.tcpConn)
	}()
	if err := misc.Pipe(conn.proxy.ReadBufferSize, conn.tcpConn, conn.tc); err != nil {
		return
	}
}

// Close and terminate the proxy TCP connection and its transmission control.
func (conn *proxyConnection) Close() error {
	conn.tc.Close()
	_ = conn.tcpConn.Close()
	return nil
}

// Proxy manages the full life cycle of multiple transmission controls created
// for the purpose of relaying TCP connections.
type Proxy struct {
	// ReadBufferSize is the buffer
	ReadBufferSize int
	// IOTimeout is the timeout shared by both TC and proxy connections.
	IOTimeout time.Duration

	connections map[uint16]*proxyConnection
	context     context.Context
	cancelFun   func()
	mutex       *sync.Mutex
	logger      lalog.Logger
}

// Start initialises the internal state of the proxy.
func (proxy *Proxy) Start(ctx context.Context) {
	if proxy.ReadBufferSize == 0 {
		// Keep the buffer size small.
		proxy.ReadBufferSize = 256
	}
	proxy.context, proxy.cancelFun = context.WithCancel(ctx)
	proxy.mutex = new(sync.Mutex)
	proxy.logger = lalog.Logger{ComponentName: "TCProxy"}
}

// Receive processes an incoming segment and relay the segment to an existing
// transmission control, or create a new transmission control for the proxy
// destination.
func (proxy *Proxy) Receive(in Segment) (resp Segment) {
	proxy.mutex.Lock()
	conn, exists := proxy.connections[in.ID]
	proxy.mutex.Unlock()
	if exists {
		// Pass the segment to transmission control's input transport.
		if _, err := conn.inputSegments.Write(in.Packet()); err != nil {
			conn.Close()
		}
	} else {
		// Connect to the proxy destination.
		var req ProxyRequest
		if err := json.Unmarshal(in.Data, &req); err != nil {
			proxy.logger.Warning("Receive", "", err, "failed to deserialise proxy request")
			return
		}
		dest := fmt.Sprintf("%s:%d", req.Address, req.Port)
		netConn, err := net.Dial("tcp", dest)
		if err != nil {
			proxy.logger.Warning("Receive", "", err, "failed to connect to proxy destination %s", dest)
		}
		// Establish the transmission control by completing the handshake.
		proxyIn, tcIn := net.Pipe()
		tc := &TransmissionControl{
			// This transmission control is a responder during the handshake.
			Initiator:      false,
			ReadTimeout:    proxy.IOTimeout,
			WriteTimeout:   proxy.IOTimeout,
			InputTransport: tcIn,
			// The output transport is not used. Instead, the output segments
			// are kept in a backlog.
			OutputTransport: io.Discard,
		}
		// Track the new connection.
		proxyConn := &proxyConnection{
			proxy:         proxy,
			tcpConn:       netConn.(*net.TCPConn),
			context:       proxy.context,
			tc:            tc,
			inputSegments: proxyIn,
		}
		proxyConn.Start()
		proxy.mutex.Lock()
		proxy.connections[in.ID] = proxyConn
		proxy.mutex.Unlock()
	}
	// TODO FIXME: pop a segment from outputSegmentBacklog
	return Segment{} // TODO FIXME
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
