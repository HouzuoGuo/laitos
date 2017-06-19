package plain

import (
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/global"
	"github.com/HouzuoGuo/laitos/ratelimit"
	"net"
	"time"
)

const (
	IOTimeoutSec         = 120 * time.Second // If a conversation goes silent for this many seconds, the connection is terminated.
	RateLimitIntervalSec = 10                // Rate limit is calculated at 10 seconds interval
	MaxConversations     = 100               // After this many successful and failed conversations, the connection is terminated.
)

// Provide access to features via plain unencrypted TCP and UDP connections.
type PlainText struct {
	TCPListenAddress string `json:"TCPListenAddress"` // TCP network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	TCPListenPort    int    `json:"TCPListenPort"`    // TCP port to listen on
	UDPListenAddress string `json:"UDPListenAddress"` // UDP network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	UDPListenPort    int    `json:"UDPListenPort"`    // UDP port to listen on

	CommandTimeoutSec int `json:"CommandTimeoutSec"` // Commands get time out error after this number of seconds
	PerIPLimit        int `json:"PerIPLimit"`        // How many times in 10 seconds interval a client IP may converse (connect/run feature) with server

	Processor *common.CommandProcessor `json:"-"` // Feature command processor
	Listener  net.Listener             `json:"-"` // Once daemon is started, this is its TCP listener.
	RateLimit *ratelimit.RateLimit     `json:"-"` // Rate limit counter per IP address
	Logger    global.Logger            `json:"-"` // Logger
}
