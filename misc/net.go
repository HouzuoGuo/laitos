package misc

import (
	"net"
	"strconv"
	"time"
)

// ProbePort makes at most 100 attempts at contacting the TCP server specified by its host and port, for up to specified
// maximum duration.
// If the TCP server accepts a connection, the connection will be immediately closed and the function will return true.
// If after the maximum duration the TCP server still has not accepted a connection, the function will return false.
func ProbePort(maxDuration time.Duration, host string, port int) bool {
	maxRounds := 100
	for i := 0; i < maxRounds; i++ {
		client, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err == nil {
			_ = client.Close()
			return true
		}
		time.Sleep(maxDuration / time.Duration(maxRounds))
	}
	return false
}

// TweakTCPConnection sets various TCP options on the connection to improve its responsiveness.
func TweakTCPConnection(conn *net.TCPConn, firstTransferTimeout time.Duration) {
	// Ask OS not to delay delivery of packet segments in hopes of bundling smaller packets into a single segment
	_ = conn.SetNoDelay(true)
	// Ask OS to periodically send smaller packets to keep the TCP connection alive, this feature of TCP works independently from application layer.
	_ = conn.SetKeepAlive(true)
	// Set keep-alive interval to every minute
	_ = conn.SetKeepAlivePeriod(60 * time.Second)
	// The first data transfer from server to client or client to server must take place before the timeout occurrs
	_ = conn.SetDeadline(time.Now().Add(firstTransferTimeout))
	// Allow outstanding data to be transferred within 5 seconds of closing the connection
	_ = conn.SetLinger(5)
}
