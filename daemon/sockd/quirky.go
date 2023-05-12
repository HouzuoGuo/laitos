package sockd

import (
	"bytes"
	"math/bits"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
)

/*
PipeTCPConnection receives data from the first connection and copies the data into the second connection.
The function returns after the first connection is closed or other IO error occurs, and before returning
the function closes the second connection and optionally writes a random amount of data into the supposedly
already terminated first connection.
*/
func PipeTCPConnection(src, dest net.Conn, doWriteRand bool) {
	defer func() {
		lalog.DefaultLogger.MaybeMinorError(dest.Close())
	}()
	buf := make([]byte, RandNum(1024, 128, 256))
	for {
		if misc.EmergencyLockDown {
			lalog.DefaultLogger.Warning("", misc.ErrEmergencyLockDown, "")
			lalog.DefaultLogger.MaybeMinorError(src.Close())
			lalog.DefaultLogger.MaybeMinorError(dest.Close())
			return
		}
		lalog.DefaultLogger.MaybeMinorError(src.SetReadDeadline(time.Now().Add(IOTimeout)))
		n, err := ReadWithRetry(src, buf)
		if err != nil {
			if doWriteRand {
				WriteRandomToTCP(src)
			}
			return
		}
		lalog.DefaultLogger.MaybeMinorError(dest.SetWriteDeadline(time.Now().Add(IOTimeout)))
		if _, err := WriteWithRetry(dest, buf[:n]); err != nil {
			return
		}
	}
}

// WriteRandomToTCP writes a random amount of data (up to couple of KB) to the connection.
func WriteRandomToTCP(conn net.Conn) (totalBytes int) {
	for i := 0; i < RandNum(1, 2, 3); i++ {
		time.Sleep(time.Duration(RandNum(170, 190, 230)) * time.Millisecond)
		lalog.DefaultLogger.MaybeMinorError(conn.SetWriteDeadline(time.Now().Add(time.Duration(RandNum(4, 5, 6)) * time.Second)))
		if n, err := conn.Write([]byte(RandomText(RandNum(290, 310, 370)))); err != nil {
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
		}
	}
	if rand.Intn(500) < 1 {
		lalog.DefaultLogger.Info(conn.RemoteAddr().String(), err, "wrote %d bytes in %d portions after %d attempts", totalWritten, maxPortions, attempts+1)
	}
	return
}

var quirkyWords = []string{
	"Accept", "Agent", "Content", "Control", "Cookie", "DOCTYPE", "DYNAMIC", "Date",
	"Directly", "Encoding", "Expires", "Found", "GET", "HEAD", "HTTP", "HttpOnly",
	"Last", "Length", "Location", "Lookup", "Mark", "Moved", "POST", "PUT", "Path",
	"Permanently", "Powered", "Report", "Return", "Status", "Strict", "Timing", "Trying",
	"Type", "UUID", "alive", "body", "bundle", "charset", "chunked", "desc", "endpoints",
	"fraction", "front", "group", "head", "html", "intact", "keep", "left", "multiuse",
	"must", "port", "public", "report", "revalidate", "store", "success", "supporting",
	"text", "title", "xhtml", "HTTP/1.1"}

var quirkyChars = []rune{'O', 'W', 'g', 'k', 'm', 'n', 's', 'u', 'v', 'y', 'z', 'o', 'w'}

// RandomText returns a string consisting of letters and spaces only.
func RandomText(length int) string {
	var pieces []string
	var gotLen int
	var wasWord bool
	for gotLen < length {
		if rand.Intn(11) <= 1 && !wasWord {
			word := quirkyWords[rand.Intn(len(quirkyWords))]
			pieces = append(pieces, word)
			gotLen += len(word)
			wasWord = true
		} else {
			var round bytes.Buffer
			roundLen := 23 + rand.Intn(29)
			for i := 0; i < roundLen; i++ {
				round.WriteRune(quirkyChars[rand.Intn(len(quirkyChars))])
			}
			pieces = append(pieces, round.String())
			gotLen += roundLen
			wasWord = false
		}
	}
	return strings.Join(pieces, " ")[:length]
}

func validateRandomQuality(t testingstub.T, txt string) (popRate float32) {
	for _, r := range txt {
		if !(r >= 65 && r <= 90 || r >= 97 && r <= 122 || r == ' ' || r == '.' || r == '/' || r == '1') {
			t.Fatalf("unexpected character: %q", r)
		}
	}
	var popCount int
	for _, c := range txt {
		popCount += bits.OnesCount(uint(c))
	}
	popRate = float32(popCount) / float32(len(txt)*8)
	if popRate < 0.6 {
		t.Fatalf("unexpected pop rate: %v - %q", popRate, txt)
	}
	return popRate
}
