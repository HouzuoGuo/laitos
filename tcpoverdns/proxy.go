package tcpoverdns

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

type ProxyRequest struct {
	Port    int    `json:"p"`
	Address string `json:"a"`
}

type proxyConnection struct {
	tcpConn              *net.TCPConn
	context              context.Context
	tc                   *TransmissionControl
	inputSegments        net.Conn
	outputSegments       net.Conn
	outputSegmentBacklog []Segment
}

func (conn *proxyConnection) Close() error {
	conn.tc.Close()
	_ = conn.tcpConn.Close()
	return nil
}

// Proxy manages the full life cycle of multiple transmission controls created
// for the purpose of relaying TCP connections.
type Proxy struct {
	// ProxyReadBufferSize is the buffer
	ProxyReadBufferSize int
	// IOTimeout is the timeout shared by both TC and proxy connections.
	IOTimeout time.Duration

	tcs         map[uint16]*TransmissionControl
	connections map[uint16]*proxyConnection
	context     context.Context
	cancelFun   func()
	mutex       *sync.Mutex
	logger      lalog.Logger
}

// Start initialises the internal state of the proxy.
func (proxy *Proxy) Start(ctx context.Context) {
	if proxy.ProxyReadBufferSize == 0 {
		// Keep the buffer size small.
		proxy.ProxyReadBufferSize = 256
	}
	proxy.tcs = make(map[uint16]*TransmissionControl)
	proxy.context, proxy.cancelFun = context.WithCancel(ctx)
	proxy.mutex = new(sync.Mutex)
	proxy.logger = lalog.Logger{ComponentName: "ProxyOverTC"}
}

// Receive processes an incoming segment and relay the segment to an existing
// transmission control, or create a new transmission control for the proxy
// destination.
func (proxy *Proxy) Receive(in Segment) (resp Segment) {
	proxy.mutex.Lock()
	conn, exists := proxy.connections[in.ID]
	proxy.mutex.Unlock()
	if exists {
		// Pass the segment to TC's input transport.
		if _, err := conn.inputSegments.Write(in.Packet()); err != nil {
			conn.Close()
		}
	} else {
		// TODO FIXME: change TC to allow the initiator to put data into the first segment.
		// TODO FIXME: deduplicate proxy requests from within a short interval.
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
		proxyOut, tcOut := net.Pipe()
		tc := &TransmissionControl{
			ReadTimeout:     proxy.IOTimeout,
			WriteTimeout:    proxy.IOTimeout,
			InputTransport:  tcIn,
			OutputTransport: tcOut,
		}
		tc.Start(proxy.context)
		// Track the new connection.
		proxyConn := &proxyConnection{
			tcpConn:        netConn.(*net.TCPConn),
			context:        proxy.context,
			tc:             tc,
			inputSegments:  proxyIn,
			outputSegments: proxyOut,
		}
		proxy.mutex.Lock()
		proxy.connections[in.ID] = proxyConn
		proxy.mutex.Unlock()
		// TODO FIXME: pipe data: read tcpConn write tc & read tc write tcpConn
	}
	// TODO FIXME: pop a segment from outputSegmentBacklog
	return Segment{} // TODO FIXME
}

// Stop terminates all ongoing transmission controls.
func (proxy *Proxy) Stop() {
	proxy.mutex.Lock()
	// Terminate all TCs.
	for _, tc := range proxy.tcs {
		tc.Close()
	}
	defer proxy.mutex.Unlock()
	// Terminate all proxied connecitons.
	proxy.cancelFun()
}
