package dnsd

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/httpclient"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/ratelimit"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	RateLimitIntervalSec       = 10   // Rate limit is calculated at 10 seconds interval
	IOTimeoutSec               = 120  // IO timeout for both read and write operations
	MaxPacketSize              = 9038 // Maximum acceptable UDP packet size
	NumQueueRatio              = 10   // Upon initialisation, create (PerIPLimit/NumQueueRatio) number of queues to handle queries.
	BlacklistUpdateIntervalSec = 7200 // Update ad-server blacklist at this interval
	MinNameQuerySize           = 14   // If a query packet is shorter than this length, it cannot possibly be a name query.
)

// A query to forward to DNS forwarder via DNS.
type UDPForwarderQuery struct {
	MyServer    *net.UDPConn
	ClientAddr  *net.UDPAddr
	DomainName  string
	QueryPacket []byte
}

// A query to forward to DNS forwarder via TCP.
type TCPForwarderQuery struct {
	MyServer    *net.Conn
	DomainName  string
	QueryPacket []byte
}

// A DNS forwarder daemon that selectively refuse to answer certain A record requests made against advertisement servers.
type DNSD struct {
	UDPListenAddress   string                    `json:"UDPListenAddress"` // UDP network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	UDPListenPort      int                       `json:"UDPListenPort"`    // UDP port to listen on
	UDPForwardTo       string                    `json:"UDPForwardTo"`     // Forward UDP DNS queries to this address (IP:Port)
	UDPForwarderConns  []net.Conn                `json:"-"`                // UDP connections made toward forwarder
	UDPForwarderQueues []chan *UDPForwarderQuery `json:"-"`                // Processing queues that handle UDP forward queries

	TCPListenAddress string `json:"TCPListenAddress"` // TCP network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	TCPListenPort    int    `json:"TCPListenPort"`    // TCP port to listen on
	TCPForwardTo     string `json:"TCPForwardTo"`     // Forward TCP DNS queries to this address (IP:Port)

	AllowQueryIPPrefixes []string `json:"AllowQueryIPPrefixes"` // Only allow queries from IP addresses that carry any of the prefixes
	PerIPLimit           int      `json:"PerIPLimit"`           // How many times in 10 seconds interval an IP may send DNS request

	RateLimit      *ratelimit.RateLimit `json:"-"` // Rate limit counter
	BlackListMutex *sync.Mutex          `json:"-"` // Protect against concurrent access to black list
	BlackList      map[string]struct{}  `json:"-"` // Do not answer to type A queries made toward these domains
	Logger         lalog.Logger         `json:"-"` // Logger
}

// Check configuration and initialise internal states.
func (dnsd *DNSD) Initialise() error {
	if dnsd.UDPListenAddress == "" && dnsd.TCPListenAddress == "" {
		return errors.New("DNSD.Initialise: listen address must not be empty")
	}
	if dnsd.UDPListenPort < 1 && dnsd.TCPListenPort < 1 {
		return errors.New("DNSD.Initialise: listen port must be greater than 0")
	}
	if dnsd.UDPForwardTo == "" && dnsd.TCPForwardTo == "" {
		return errors.New("DNSD.Initialise: the server is not useful if UDPForwardTo address is empty")
	}
	if dnsd.PerIPLimit < 10 {
		return errors.New("DNSD.Initialise: PerIPLimit must be greater than 9")
	}
	if len(dnsd.AllowQueryIPPrefixes) == 0 {
		return errors.New("DNSD.Initialise: allowable IP prefixes list must not be empty")
	}
	for _, prefix := range dnsd.AllowQueryIPPrefixes {
		if prefix == "" {
			return errors.New("DNSD.Initialise: any allowable IP prefixes must not be empty string")
		}
	}
	dnsd.BlackListMutex = new(sync.Mutex)
	dnsd.BlackList = make(map[string]struct{})
	dnsd.RateLimit = &ratelimit.RateLimit{
		MaxCount: dnsd.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
		Logger:   dnsd.Logger,
	}
	dnsd.RateLimit.Initialise()
	// Create a number of forwarder queues to handle incoming UDP DNS queries
	numQueues := dnsd.PerIPLimit / NumQueueRatio
	dnsd.UDPForwarderConns = make([]net.Conn, numQueues)
	dnsd.UDPForwarderQueues = make([]chan *UDPForwarderQuery, numQueues)
	for i := 0; i < numQueues; i++ {
		forwarderAddr, err := net.ResolveUDPAddr("udp", dnsd.UDPForwardTo)
		if err != nil {
			return fmt.Errorf("DNSD.Initialise: failed to resolve address - %v", err)
		}
		forwarderConn, err := net.DialTimeout("udp", forwarderAddr.String(), IOTimeoutSec*time.Second)
		if err != nil {
			return fmt.Errorf("DNSD.Initialise: failed to connect to forwarder - %v", err)
		}
		dnsd.UDPForwarderConns[i] = forwarderConn
		dnsd.UDPForwarderQueues[i] = make(chan *UDPForwarderQuery, 16) // there really is no need for a deeper queue
	}
	return nil
}

// Download ad-servers list from yoyo.org and put them into blacklist.
func (dnsd *DNSD) InstallAdBlacklist() (int, error) {
	yoyo := "http://pgl.yoyo.org/adservers/serverlist.php?hostformat=nohtml&showintro=0&mimetype=plaintext"
	resp, err := httpclient.DoHTTP(httpclient.Request{TimeoutSec: 30}, yoyo)
	if err != nil {
		return 0, err
	}
	if statusErr := resp.Non2xxToError(); statusErr != nil {
		return 0, statusErr
	}
	adServerNames := strings.Split(string(resp.Body), "\n")
	if len(adServerNames) < 100 {
		return 0, fmt.Errorf("yoyo's ad-server list is suspiciously short at only %d lines", len(adServerNames))
	}
	dnsd.BlackListMutex.Lock()
	defer dnsd.BlackListMutex.Unlock()
	dnsd.BlackList = make(map[string]struct{})
	for _, name := range adServerNames {
		dnsd.BlackList[strings.TrimSpace(name)] = struct{}{}
	}
	return len(dnsd.BlackList), nil
}

var StandardResponseNoError = []byte{129, 128} // DNS response packet flag - standard response, no indication of error.

//                            Domain     A    IN      TTL 1466  IPv4     0.0.0.0
var BlackHoleAnswer = []byte{192, 12, 0, 1, 0, 1, 0, 0, 5, 186, 0, 4, 0, 0, 0, 0} // DNS answer 0.0.0.0

// Create a DNS response packet without prefix length bytes, that points incoming query to 0.0.0.0.
func RespondWith0(queryNoLength []byte) []byte {
	if queryNoLength == nil || len(queryNoLength) < MinNameQuerySize {
		return []byte{}
	}
	answerPacket := make([]byte, 2+2+len(queryNoLength)-4+len(BlackHoleAnswer))
	// Match transaction ID of original query
	answerPacket[0] = queryNoLength[0]
	answerPacket[1] = queryNoLength[1]
	// 0x8180 - response is a standard query response, without indication of error.
	copy(answerPacket[2:4], StandardResponseNoError)
	// Copy of original query structure
	copy(answerPacket[4:], queryNoLength[4:])
	// There is exactly one answer RR
	answerPacket[6] = 0
	answerPacket[7] = 1
	// Answer 0.0.0.0 to the query
	copy(answerPacket[len(answerPacket)-len(BlackHoleAnswer):], BlackHoleAnswer)
	// Finally, respond!
	return answerPacket
}

// Extract domain name from type A class IN query. Return empty string if query packet does not look like A-IN query.
func ExtractDomainName(packet []byte) string {
	if packet == nil || len(packet) < MinNameQuerySize {
		return ""
	}
	indexTypeAClassIN := bytes.Index(packet[13:], []byte{0, 1, 0, 1})
	if indexTypeAClassIN < 1 {
		return ""
	}
	indexTypeAClassIN += 13
	// The byte right before Type-A Class-IN is an empty byte to be discarded
	domainNameBytes := make([]byte, indexTypeAClassIN-13-1)
	copy(domainNameBytes, packet[13:indexTypeAClassIN-1])
	/*
		Restore full-stops of the domain name portion so that it can be checked against black list.
		Not sure why those byte ranges show up in place of full-stops, probably due to some RFCs.
	*/
	for i, b := range domainNameBytes {
		if b <= 44 || b >= 58 && b <= 64 || b >= 91 && b <= 96 {
			domainNameBytes[i] = '.'
		}
	}
	return string(domainNameBytes)
}

/*
You may call this function only after having called Initialise()!
Start DNS daemon on both UDP and TCP ports, block until this program exits.
*/
func (dnsd *DNSD) StartAndBlock() error {
	// Keep updating ad-block black list in background
	go func() {
		for {
			if numEntries, err := dnsd.InstallAdBlacklist(); err == nil {
				dnsd.Logger.Printf("InstallAdBlacklist", "", nil, "successfully updated ad-blacklist with %d entries", numEntries)
			} else {
				dnsd.Logger.Printf("InstallAdBlacklist", "", err, "failed to update ad-blacklist")
			}
			time.Sleep(BlacklistUpdateIntervalSec * time.Second)
		}
	}()
	errChan := make(chan error, 2)
	if dnsd.UDPListenPort != 0 {
		go func() {
			if err := dnsd.StartAndBlockUDP(); err != nil {
				errChan <- err
			}
		}()
	}
	if dnsd.TCPListenPort != 0 {
		go func() {
			if err := dnsd.StartAndBlockTCP(); err != nil {
				errChan <- err
			}
		}()
	}
	return <-errChan
}
