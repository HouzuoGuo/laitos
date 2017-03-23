package dnsd

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/httpclient"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/ratelimit"
	"math/rand"
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
)

// A query to forward to DNS forwarder.
type ForwarderQuery struct {
	MyServer    *net.UDPConn
	ClientAddr  *net.UDPAddr
	DomainName  string
	QueryPacket []byte
}

// A DNS forwarder daemon that selectively refuse to answer certain A record requests made against advertisement servers.
type DNSD struct {
	ListenAddress        string   `json:"ListenAddress"`        // Network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	ListenPort           int      `json:"ListenPort"`           // Port to listen on
	ForwardTo            string   `json:"ForwardTo"`            // Forward DNS queries to this address
	AllowQueryIPPrefixes []string `json:"AllowQueryIPPrefixes"` // Only allow queries from IP addresses that carry any of the prefixes
	PerIPLimit           int      `json:"PerIPLimit"`           // How many times in 10 seconds interval an IP may send DNS request

	RateLimit       *ratelimit.RateLimit   `json:"-"` // Rate limit counter
	BlackListMutex  *sync.Mutex            `json:"-"` // Protect against concurrent access to black list
	BlackList       map[string]struct{}    `json:"-"` // Do not answer to type A queries made toward these domains
	ForwarderConns  []net.Conn             `json:"-"` // UDP connections made toward forwarder
	ForwarderQueues []chan *ForwarderQuery `json:"-"` // Processing queues that handle forward queries
	Logger          lalog.Logger           `json:"-"` // Logger
}

// Send forward queries to forwarder and forward the response to my DNS client.
func (dnsd *DNSD) ForwarderQueueProcessor(myQueue chan *ForwarderQuery, forwarderConn net.Conn) {
	packetBuf := make([]byte, MaxPacketSize)
	for {
		query := <-myQueue
		// Set deadline for IO with forwarder
		forwarderConn.SetDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		if _, err := forwarderConn.Write(query.QueryPacket); err != nil {
			dnsd.Logger.Printf("ForwarderQueueProcessor", "Write", err, "IO failure")
			continue
		}
		packetLength, err := forwarderConn.Read(packetBuf)
		if err != nil {
			dnsd.Logger.Printf("ForwarderQueueProcessor", "Read", err, "IO failure")
			continue
		}
		// Set deadline for responding to my DNS client
		query.MyServer.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		if _, err := query.MyServer.WriteTo(packetBuf[:packetLength], query.ClientAddr); err != nil {
			dnsd.Logger.Printf("ForwarderQueueProcessor", "WriteResponse", err, "IO failure")
			continue
		}
		dnsd.Logger.Printf("ForwarderQueueProcessor", query.ClientAddr.IP.String(), nil,
			"successfully forwarded answer for \"%s\", backlog length %d", query.DomainName, len(myQueue))
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
	dnsd.BlackListMutex = new(sync.Mutex)
	dnsd.BlackList = make(map[string]struct{})
	dnsd.RateLimit = &ratelimit.RateLimit{
		MaxCount: dnsd.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
		Logger:   dnsd.Logger,
	}
	dnsd.RateLimit.Initialise()
	// Create a number of forwarder queues to handle incoming DNS queries
	numQueues := dnsd.PerIPLimit / NumQueueRatio
	dnsd.ForwarderConns = make([]net.Conn, numQueues)
	dnsd.ForwarderQueues = make([]chan *ForwarderQuery, numQueues)
	for i := 0; i < numQueues; i++ {
		forwarderAddr, err := net.ResolveUDPAddr("udp", dnsd.ForwardTo+":53")
		if err != nil {
			return fmt.Errorf("DNSD.Initialise: failed to resolve address - %v", err)
		}
		forwarderConn, err := net.DialTimeout("udp", forwarderAddr.String(), IOTimeoutSec*time.Second)
		if err != nil {
			return fmt.Errorf("DNSD.Initialise: failed to connect to forwarder - %v", err)
		}
		dnsd.ForwarderConns[i] = forwarderConn
		dnsd.ForwarderQueues[i] = make(chan *ForwarderQuery, 16) // there really is no need for a deeper queue
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

// Respond to the DNS query with a black-hole answer to 0.0.0.0.
func (dnsd *DNSD) RespondWith0(myServer *net.UDPConn, clientAddr *net.UDPAddr, queryPacket []byte) {
	answerPacket := make([]byte, 2+2+len(queryPacket)-4+len(BlackHoleAnswer))
	// Match transaction ID of original query
	answerPacket[0] = queryPacket[0]
	answerPacket[1] = queryPacket[1]
	// 0x8180 - response is a standard query response, without indication of error.
	copy(answerPacket[2:4], StandardResponseNoError)
	// Copy of original query structure
	copy(answerPacket[4:], queryPacket[4:])
	// There is exactly one answer RR
	answerPacket[6] = 0
	answerPacket[7] = 1
	// Answer 0.0.0.0 to the query
	copy(answerPacket[len(answerPacket)-len(BlackHoleAnswer):], BlackHoleAnswer)
	// Finally, respond!
	myServer.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second))
	if _, err := myServer.WriteTo(answerPacket, clientAddr); err != nil {
		dnsd.Logger.Printf("RespondWith0", clientAddr.IP.String(), err, "IO failure")
	}
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
	// Start forwarder queues that will forward client queries and respond to them
	for i, queue := range dnsd.ForwarderQueues {
		go dnsd.ForwarderQueueProcessor(queue, dnsd.ForwarderConns[i])
	}
	// Dispatch queries to forwarder queues
	packetBuf := make([]byte, MaxPacketSize)
	dnsd.Logger.Printf("StartAndBlock", "", nil, "going to listen for queries")
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
			dnsd.Logger.Printf("Loop", clientIP, nil, "client IP is not allowed by configuration")
			continue
		}

		// Prepare parameters for forwarding the query
		indexTypeAClassIN := -1
		if packetLength > 14 {
			indexTypeAClassIN = 14 + bytes.Index(packetBuf[14:packetLength], []byte{0, 1, 0, 1})
		}
		randForwarder := rand.Intn(len(dnsd.ForwarderQueues))
		forwardPacket := make([]byte, packetLength)
		copy(forwardPacket, packetBuf[:packetLength])
		var domainName string
		if indexTypeAClassIN > 14 {
			// This is a domain name query, check the name against black list and then forward.
			domainNameBytes := make([]byte, indexTypeAClassIN-13-1)
			copy(domainNameBytes, packetBuf[13:indexTypeAClassIN-1])
			/*
				Restore full-stops of the domain name portion so that it can be checked against black list.
				Not sure why those byte ranges show up in place of full-stops, probably due to some RFCs.
			*/
			for i, b := range domainNameBytes {
				if b <= 44 || b >= 58 && b <= 64 || b >= 91 && b <= 96 {
					domainNameBytes[i] = '.'
				}
			}
			domainName = string(domainNameBytes)
			// Do not respond to black list domain names
			dnsd.BlackListMutex.Lock()
			_, blacklisted := dnsd.BlackList[domainName]
			dnsd.BlackListMutex.Unlock()
			if blacklisted {
				dnsd.Logger.Printf("Loop", clientIP, nil, "answer to black-listed domain \"%s\"", domainName)
				dnsd.RespondWith0(udpServer, clientAddr, forwardPacket)
				continue
			} else {
				dnsd.Logger.Printf("Loop", clientIP, nil, "let forwarder %d handle domain \"%s\"", randForwarder, domainName)
				// Forwarder queue will take care of this query
			}
		} else {
			// If query is not about domain name, simply forward it without much concern.
			dnsd.Logger.Printf("Loop", clientIP, nil, "let forwarder %d handle non-name query", randForwarder)
		}
		dnsd.ForwarderQueues[randForwarder] <- &ForwarderQuery{
			ClientAddr:  clientAddr,
			DomainName:  domainName,
			MyServer:    udpServer,
			QueryPacket: forwardPacket,
		}
	}
}
