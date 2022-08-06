package misc

import (
	"net"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

// ProbePort makes at most 100 attempts at contacting the TCP server specified
// by its host and port, for up to specified maximum duration.
// If the TCP server accepts a connection, the connection will be immediately
// closed and the function will return true.
// If after the maximum duration the TCP server still has not accepted a
// connection, the function will return false and print a warning log message.
func ProbePort(maxDuration time.Duration, host string, port int) bool {
	maxRounds := 100
	start := time.Now()
	for i := 0; i < maxRounds; i++ {
		client, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err == nil {
			_ = client.Close()
			return true
		}
		if time.Now().Sub(start) > maxDuration {
			goto fail
		}
		time.Sleep(maxDuration / time.Duration(maxRounds))
	}
fail:
	lalog.DefaultLogger.Warning("ProbePort", "", nil, "%s:%d did not respond within %s. Stack: %s", host, port, maxDuration, debug.Stack())
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
	// The first data transfer from server to client or client to server must take place before the timeout occurs
	_ = conn.SetDeadline(time.Now().Add(firstTransferTimeout))
	// Allow outstanding data to be transferred within 5 seconds of closing the connection
	_ = conn.SetLinger(5)
}

// PipeConn continuously reads data from the source net connection in blocks of
// no more than the specified buffer length, and writes them to the destination
// connection.
func PipeConn(logger lalog.Logger, autoClose bool, ioTimeout time.Duration, bufLen int, src, dest net.Conn) error {
	if autoClose {
		defer func() {
			logger.MaybeMinorError(src.Close())
			logger.MaybeMinorError(dest.Close())
		}()
	}
	buf := make([]byte, bufLen)
	for {
		if EmergencyLockDown {
			logger.Warning("Pipe", "", ErrEmergencyLockDown, "")
			logger.MaybeMinorError(src.Close())
			logger.MaybeMinorError(dest.Close())
			return nil
		}
		logger.MaybeMinorError(src.SetReadDeadline(time.Now().Add(ioTimeout)))
		n, err := src.Read(buf)
		if err != nil {
			return err
		}
		logger.MaybeMinorError(dest.SetWriteDeadline(time.Now().Add(ioTimeout)))
		if _, err := dest.Write(buf[:n]); err != nil {
			return err
		}
	}
}
