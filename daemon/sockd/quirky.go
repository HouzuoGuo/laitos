package sockd

import (
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

/*
PipeTCPConnection receives data from the first connection and copies the data into the second connection.
The function returns after the first connection is closed or other IO error occurs, and before returning
the function closes the second connection and optionally writes a random amount of data into the supposedly
already terminated first connection.
*/
func PipeTCPConnection(fromConn, toConn net.Conn, doWriteRand bool) {
	defer func() {
		_ = toConn.Close()
	}()
	// Read and write a small TCP segment at a time to avoid IP fragmentation
	buf := make([]byte, RandNum(1024, 128, 256))
	for {
		if misc.EmergencyLockDown {
			lalog.DefaultLogger.Warning("sockd", misc.ErrEmergencyLockDown, "")
			return
		} else if err := fromConn.SetReadDeadline(time.Now().Add(IOTimeout)); err != nil {
			return
		}
		length, err := ReadWithRetry(fromConn, buf)
		if length > 0 {
			if err := toConn.SetWriteDeadline(time.Now().Add(IOTimeout)); err != nil {
				return
			} else if _, err := WriteWithRetry(toConn, buf[:length]); err != nil {
				return
			}
		}
		if err != nil {
			if doWriteRand {
				WriteRandomToTCP(fromConn)
			}
			return
		}
	}
}

// WriteRandomToTCP writes a random amount of data (up to couple of KB) to the connection.
func WriteRandomToTCP(conn net.Conn) (totalBytes int) {
	for i := 0; i < RandNum(1, 2, 3); i++ {
		time.Sleep(time.Duration(RandNum(890, 1440, 2330)) * time.Millisecond)
		lalog.DefaultLogger.MaybeMinorError(conn.SetWriteDeadline(time.Now().Add(time.Duration(RandNum(5, 6, 7)) * time.Second)))
		if n, err := conn.Write([]byte(RandomText(RandNum(210, 340, 550)))); err != nil {
			lalog.DefaultLogger.MaybeMinorError(err)
			break
		} else {
			totalBytes += n
		}
	}
	if rand.Intn(100) < 2 {
		lalog.DefaultLogger.Info(conn.RemoteAddr().String(), nil, "wrote %d rand bytes", totalBytes)
	}
	return
}

func WriteRandomToUDP(srv *net.UDPConn, client *net.UDPAddr) (totalBytes int) {
	time.Sleep(time.Duration(RandNum(780, 900, 1200)) * time.Millisecond)
	lalog.DefaultLogger.MaybeMinorError(srv.SetWriteDeadline(time.Now().Add(time.Duration(RandNum(1, 2, 3)) * time.Second)))
	var err error
	if totalBytes, err = srv.WriteToUDP([]byte(RandomText(RandNum(4, 5, 60))), client); err != nil {
		lalog.DefaultLogger.MaybeMinorError(err)
		return
	}
	if rand.Intn(100) < 2 {
		lalog.DefaultLogger.Info(client.IP.String(), nil, "wrote %d rand bytes", totalBytes)
	}
	return
}

// ReadWithRetry makes at most 3 attempts to read incoming data from the connection. If an IO error occurs, the connection will be closed.
func ReadWithRetry(conn net.Conn, buf []byte) (n int, err error) {
	attempts := 0
	for ; attempts < 3; attempts++ {
		if err = conn.SetReadDeadline(time.Now().Add(IOTimeout)); err == nil {
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
	if rand.Intn(500) < 1 {
		lalog.DefaultLogger.Info(conn.RemoteAddr().String(), err, "read %d bytes after %d attempts", n, attempts+1)
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
			if err = conn.SetWriteDeadline(time.Now().Add(IOTimeout)); err == nil {
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
	if rand.Intn(500) < 1 {
		lalog.DefaultLogger.Info(conn.RemoteAddr().String(), err, "wrote %d bytes in %d portions after %d attempts", totalWritten, maxPortions, attempts+1)
	}
	return
}

// RandomText returns a string consisting of letters only.
func RandomText(length int) string {
	var ret []rune
	for i := 0; i < length/5+1; i++ {
		var r rune
		if n := rand.Intn(26 * 2); n < 26 {
			r = rune('A' + n)
		} else {
			r = rune('a' + n - 26)
		}
		// "~50%"
		ret = append(ret, r, r, r, r, r)
	}
	rand.Shuffle(len(ret), func(i, j int) {
		tmp := ret[i]
		ret[i] = ret[j]
		ret[j] = tmp
	})
	return string(ret[:length])
}
