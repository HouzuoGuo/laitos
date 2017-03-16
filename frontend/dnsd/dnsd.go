package named

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/ratelimit"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"
)

const (
	RateLimitIntervalSec = 10    // Rate limit is calculated at 10 seconds interval
	IOTimeoutSec         = 60    // IO timeout for both read and write operations
	MaxPacketSize        = 65535 // Maximum acceptable UDP packet size
	NumQueueRatio        = 10    // Upon initialisation, create (PerIPLimit/NumQueueRatio) number of queues to handle queries.
)

// A query to forward to DNS forwarder.
type ForwarderQuery struct {
	MyServer    *net.UDPConn
	ClientAddr  *net.UDPAddr
	DomainName  string
	QueryPacket []byte
}

// A DNS forwarder daemon that selectively refuse to answer certain A record requests based on a black list.
type DNSD struct {
	ListenAddress        string   `json:"ListenAddr"`           // Network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	ListenPort           int      `json:"ListenPort"`           // Port to listen on
	ForwardTo            string   `json:"ForwardTo"`            // Forward DNS queries to this address
	AllowQueryIPPrefixes []string `json:"AllowQueryIPPrefixes"` // Only allow queries from IP addresses that carry any of the prefixes
	PerIPLimit           int      `json:"PerIPLimit"`           // How many times in 10 seconds interval an IP may send DNS request

	RateLimit       *ratelimit.RateLimit   `json:"-"` // Rate limit counter
	BlackList       map[string]struct{}    `json:"-"` // Do not answer to type A queries made toward these domains
	ForwarderConns  []*net.UDPConn         `json:"-"` // UDP connections made toward forwarder
	ForwarderQueues []chan *ForwarderQuery `json:"-"` // Processing queues that handle forward queries
}

// Send forward queries to forwarder and forward the response to my DNS client.
func (dnsd *DNSD) ForwarderQueueProcessor(myQueue chan *ForwarderQuery, forwarderConn *net.UDPConn) {
	packetBuf := make([]byte, MaxPacketSize)
	for {
		query := <-myQueue
		// Set deadline for IO with forwarder
		forwarderConn.SetDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		if _, err := forwarderConn.Write(query.QueryPacket); err != nil {
			log.Printf("DNSD Queue: failed to write to forwarder - %v", err)
			continue
		}
		packetLength, err := forwarderConn.Read(packetBuf)
		if err != nil {
			log.Printf("DNSD Queue: failed to read from forwarder - %v", err)
			continue
		}
		// Set deadline for responding to my DNS client
		query.MyServer.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		if _, err := query.MyServer.WriteTo(packetBuf[:packetLength], query.ClientAddr); err != nil {
			log.Printf("DNSD Queue: failed to respond to client - %v", err)
			continue
		}

		log.Printf("DNSD: successfully responded to %s regarding \"%s\", backlog length: %d",
			query.ClientAddr.IP.String(), query.DomainName, len(myQueue))
	}
}

// Check configuration and initialise internal states.
func (dnsd *DNSD) Initialise() error {
	if dnsd.ListenAddress == "" {
		return errors.New("DNSD.Initialise: listen address must not be empty")
	}
	if dnsd.ListenPort < 1 {
		return errors.New("DNSD.Initialise: listen port must be greater than 0")
	}
	if dnsd.ForwardTo == "" {
		return errors.New("DNSD.Initialise: the server is not useful if ForwardTo address is empty")
	}
	if dnsd.PerIPLimit < 10 {
		return errors.New("DNSD.Initialise: PerIPLimit must be greater than 9")
	}
	if len(dnsd.AllowQueryIPPrefixes) == 0 {
		return errors.New("DNSD.Initialise: allowable IP prefixes list must not be empty")
	}
	for _, prefix := range dnsd.AllowQueryIPPrefixes {
		if prefix == "" {
			return errors.New("DNSD.Initialise: all allowable IP prefixes must not be empty string")
		}
	}

	dnsd.BlackList = make(map[string]struct{}) // TODO: initialise this from ad block source
	dnsd.RateLimit = &ratelimit.RateLimit{
		MaxCount: dnsd.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
	}
	dnsd.RateLimit.Initialise()
	// Create a number of forwarder queues to handle incoming DNS queries
	numQueues := dnsd.PerIPLimit / NumQueueRatio
	dnsd.ForwarderConns = make([]*net.UDPConn, numQueues)
	dnsd.ForwarderQueues = make([]chan *ForwarderQuery, numQueues)
	for i := 0; i < numQueues; i++ {
		forwarderAddr, err := net.ResolveUDPAddr("udp", dnsd.ForwardTo+":53")
		if err != nil {
			return fmt.Errorf("DNSD.Initialise: failed to resolve address - %v", err)
		}
		forwarderConn, err := net.DialUDP("udp", nil, forwarderAddr)
		if err != nil {
			return fmt.Errorf("DNSD.Initialise: failed to connect to forwarder - %v", err)
		}
		dnsd.ForwarderConns[i] = forwarderConn
		dnsd.ForwarderQueues[i] = make(chan *ForwarderQuery, 16) // there really is no need for a deeper queue
	}
	return nil
}

/*
You may call this function only after having called Initialise()!
Start DNS daemon and block until this program exits.
*/
func (dnsd *DNSD) StartAndBlock() error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", dnsd.ListenAddress, dnsd.ListenPort))
	if err != nil {
		return err
	}
	udpServer, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	// Start forwarder queues that will forward client queries and respond to them
	for i, queue := range dnsd.ForwarderQueues {
		go dnsd.ForwarderQueueProcessor(queue, dnsd.ForwarderConns[i])
	}
	// Dispatch queries to forwarder queues
	packetBuf := make([]byte, MaxPacketSize)
	for {
		packetLength, clientAddr, err := udpServer.ReadFromUDP(packetBuf)
		if err != nil {
			return err
		}
		// Check address against rate limit
		clientIP := clientAddr.IP.String()
		if !dnsd.RateLimit.Add(clientIP, true) {
			continue
		}
		// Check address against allowed IP prefixes
		var prefixOK bool
		for _, prefix := range dnsd.AllowQueryIPPrefixes {
			if strings.HasPrefix(clientIP, prefix) {
				prefixOK = true
				break
			}
		}
		if !prefixOK {
			log.Printf("DNSD: client %s is not not allowed", clientIP)
			continue
		}
		isDomainNameQuery := bytes.HasSuffix(packetBuf[:packetLength], []byte{0, 1, 0, 1})
		// 12 bytes come before domain name, 5 bytes come after domain name
		if isDomainNameQuery && packetLength > 18 {
			// This is a domain name query, check the name against black list and then forward.
			domainNameBytes := make([]byte, packetLength-5-13)
			copy(domainNameBytes, packetBuf[13:packetLength-5])
			/*
				Restore full-stops of the domain name portion so that it can be checked against black list.
				Not sure why those byte ranges show up in place of full-stops, probably due to some RFCs.
			*/
			for i, b := range domainNameBytes {
				if b <= 44 || b >= 58 && b <= 64 || b >= 91 && b <= 96 {
					domainNameBytes[i] = '.'
				}
			}
			domainName := string(domainNameBytes)
			// Do not respond to black list domain names
			if _, exists := dnsd.BlackList[domainName]; exists {
				log.Printf("DNSD: client %s queried black list domain \"%s\"", clientIP, domainName)
			} else {
				randForwarder := rand.Intn(len(dnsd.ForwarderQueues))
				log.Printf("DNSD: let forwarder %d handle client %s \"%s\"", randForwarder, clientIP, domainName)
				// Make a copy of the packet so that subsequent DNS queries don't overwrite the buffer
				forwardPacket := make([]byte, packetLength)
				copy(forwardPacket, packetBuf[:packetLength])
				dnsd.ForwarderQueues[randForwarder] <- &ForwarderQuery{
					ClientAddr:  clientAddr,
					DomainName:  string(domainName),
					MyServer:    udpServer,
					QueryPacket: forwardPacket,
				}
			}
		} else {
			// If query is not about domain name, simply forward it without much concern.
			log.Printf("DNSD: let forwarder handle client %s's non-name query", clientIP)
		}
	}
}
