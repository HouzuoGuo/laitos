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
