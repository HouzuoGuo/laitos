package sockd

import (
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

// TweakTCPConnection tweaks the TCP connection settings for improved responsiveness.
func TweakTCPConnection(conn *net.TCPConn) {
	_ = conn.SetNoDelay(true)
	_ = conn.SetKeepAlive(true)
	_ = conn.SetKeepAlivePeriod(60 * time.Second)
	_ = conn.SetDeadline(time.Now().Add(time.Duration(IOTimeoutSec * time.Second)))
	_ = conn.SetLinger(5)
}

// WriteRand writes a random amount of data (up to couple of KB) to the connection.
func WriteRand(conn net.Conn) {
	randBytesWritten := 0
	for i := 0; i < RandNum(1, 2, 3); i++ {
		randBuf := make([]byte, RandNum(210, 340, 550))
		if _, err := rand.Read(randBuf); err != nil {
			break
		}
		time.Sleep(time.Duration(RandNum(890, 1440, 2330)) * time.Millisecond)
		// This is not the ordinary data transfer and does not require long IO timeout
		if err := conn.SetWriteDeadline(time.Now().Add(6 * time.Second)); err != nil {
			break
		}
		if n, err := conn.Write(randBuf); err != nil && !strings.Contains(err.Error(), "closed") && !strings.Contains(err.Error(), "broken") {
			break
		} else {
			randBytesWritten += n
		}
	}
	if rand.Intn(100) < 2 {
		lalog.DefaultLogger.Info("sockd.quirky.WriteRand", conn.RemoteAddr().String(), nil, "wrote %d rand bytes", randBytesWritten)
	}
}

// ReadWithRetry makes at most 3 attempts to read incoming data from the connection. If an IO error occurs, the connection will be closed.
func ReadWithRetry(conn net.Conn, buf []byte) (n int, err error) {
	attempts := 0
	for ; attempts < 3; attempts++ {
		if err = conn.SetReadDeadline(time.Now().Add(IOTimeoutSec * time.Second)); err == nil {
			if n, err = conn.Read(buf); err == nil {
				break
			} else if strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "broken") {
				break
			} else if n > 0 {
				// IO error occurred after data is partially read, the data stream is now broken.
				_ = conn.Close()
				break
			}
		}
		// Sleep couple of seconds in between attempts
		time.Sleep(time.Duration((attempts+1)*500) * time.Millisecond)
	}
	if rand.Intn(100) < 2 {
		lalog.DefaultLogger.Info("sockd.quirky.ReadWithRetry", conn.RemoteAddr().String(), err, "read %d bytes after %d attempts", n, attempts+1)
	}
	return
}

// WriteWithRetry divides the data buffer into several portions and makes at most 3 attempts to deliver each portion. If an IO error occurs, the connection will be closed.
func WriteWithRetry(conn net.Conn, buf []byte) (totalWritten int, err error) {
	attempts := 0
	maxPortions := RandNum(1, 0, 4)
	portionBufSize := len(buf) / maxPortions
	// Divide the incoming buffer into several portion
dataTransfer:
	for portion := 0; portion < maxPortions; portion++ {
		bufStart := portion * portionBufSize
		bufEnd := (portion + 1) * portionBufSize
		if portion == maxPortions-1 {
			bufEnd = len(buf)
		}
		// Make at most 3 attempts to transfer each portion
		for ; attempts < 3; attempts++ {
			if err = conn.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second)); err == nil {
				writtenBytes := 0
				if writtenBytes, err = conn.Write(buf[bufStart:bufEnd]); err == nil {
					totalWritten += writtenBytes
					break
				} else if strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "broken") {
					break dataTransfer
				} else if writtenBytes > 0 {
					// IO error occurred after data is partially written, the data stream is now broken.
					_ = conn.Close()
					break dataTransfer
				}
			}
			// Sleep couple of seconds in between attempts
			time.Sleep(time.Duration((attempts+1)*500) * time.Millisecond)
		}
		// Sleep couple of milliseconds in between each portion
		time.Sleep(time.Duration(RandNum(1, 0, maxPortions)) * time.Millisecond)
	}
	if rand.Intn(100) < 2 {
		lalog.DefaultLogger.Info("sockd.quirky.WriteWithRetry", conn.RemoteAddr().String(), err, "wrote %d bytes in %d portions after %d attempts", totalWritten, maxPortions, attempts+1)
	}
	return
}
