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

// Send forward queries to forwarder and forward the response to my DNS client.
func (dnsd *DNSD) ForwarderQueueProcessor(myQueue chan *UDPForwarderQuery, forwarderConn net.Conn) {
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
	indexTypeAClassIN := -1
	indexTypeAClassIN = 14 + bytes.Index(packet[14:], []byte{0, 1, 0, 1})
	if indexTypeAClassIN == -1 {
		return ""
	}
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
Start DNS daemon to listen on UDP port only. Block caller.
*/
func (dnsd *DNSD) StartAndBlockUDP() error {
	listenAddr := fmt.Sprintf("%s:%d", dnsd.UDPListenAddress, dnsd.UDPListenPort)
	udpAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return err
	}
	udpServer, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	// Start forwarder queues that will forward client queries and respond to them
	for i, queue := range dnsd.UDPForwarderQueues {
		go dnsd.ForwarderQueueProcessor(queue, dnsd.UDPForwarderConns[i])
	}
	// Dispatch queries to forwarder queues
	packetBuf := make([]byte, MaxPacketSize)
	dnsd.Logger.Printf("StartAndBlockUDP", listenAddr, nil, "going to listen for queries")
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
			dnsd.Logger.Printf("UDPLoop", clientIP, nil, "client IP is not allowed by configuration")
			continue
		}

		// Prepare parameters for forwarding the query
		randForwarder := rand.Intn(len(dnsd.UDPForwarderQueues))
		forwardPacket := make([]byte, packetLength)
		copy(forwardPacket, packetBuf[:packetLength])
		domainName := ExtractDomainName(forwardPacket)
		if domainName == "" {
			// If I cannot figure out what domain is from the query, simply forward it without much concern.
			dnsd.Logger.Printf("UDPLoop", clientIP, nil, "let forwarder %d handle non-name query", randForwarder)

		} else {
			// This is a domain name query, check the name against black list and then forward.
			dnsd.BlackListMutex.Lock()
			_, blacklisted := dnsd.BlackList[domainName]
			dnsd.BlackListMutex.Unlock()
			if blacklisted {
				dnsd.Logger.Printf("UDPLoop", clientIP, nil, "answer to black-listed domain \"%s\"", domainName)
				blackHoleAnswer := RespondWith0(forwardPacket)
				udpServer.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second))
				if _, err := udpServer.WriteTo(blackHoleAnswer, clientAddr); err != nil {
					dnsd.Logger.Printf("UDPLoop", clientAddr.IP.String(), err, "IO failure")
				}

				continue
			} else {
				dnsd.Logger.Printf("UDPLoop", clientIP, nil, "let forwarder %d handle domain \"%s\"", randForwarder, domainName)
				// Forwarder queue will take care of this query
			}
		}
		dnsd.UDPForwarderQueues[randForwarder] <- &UDPForwarderQuery{
			ClientAddr:  clientAddr,
			DomainName:  domainName,
			MyServer:    udpServer,
			QueryPacket: forwardPacket,
		}
	}
}

func (dnsd *DNSD) HandleTCPQuery(clientConn net.Conn) {
	defer clientConn.Close()
	var err error
	var domainName string
	var queryLen int
	var queryLenBuf []byte
	var queryBuf []byte
	var responseLen int
	var responseLenBuf []byte
	var responseBuf []byte
	var doForward bool
	// Check address against rate limit
	clientIP := clientConn.RemoteAddr().String()[:strings.LastIndexByte(clientConn.RemoteAddr().String(), ':')]
	if !dnsd.RateLimit.Add(clientIP, true) {
		return
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
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "client IP is not allowed by configuration")
		return
	}
	// Read query length
	clientConn.SetDeadline(time.Now().Add(IOTimeoutSec * time.Second))
	queryLenBuf = make([]byte, 2)
	_, err = clientConn.Read(queryLenBuf)
	if err != nil {
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error")
		return
	}
	queryLen = int(queryLenBuf[0])*256 + int(queryLenBuf[1])
	// Read query
	if queryLen > MaxPacketSize || queryLen < 1 {
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "bad query length")
		return
	}
	queryBuf = make([]byte, queryLen)
	_, err = clientConn.Read(queryBuf)
	if err != nil {
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error")
		return
	}
	domainName = ExtractDomainName(queryBuf)
	// Formulate response
	if domainName == "" {
		// If I cannot figure out what domain is from the query, simply forward it without much concern.
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "let forwarder handle non-name query")
		doForward = true
	} else {
		// This is a domain name query, check the name against black list and then forward.
		dnsd.BlackListMutex.Lock()
		_, blacklisted := dnsd.BlackList[domainName]
		dnsd.BlackListMutex.Unlock()
		if blacklisted {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "answer to black-listed domain \"%s\"", domainName)
			responseBuf = RespondWith0(queryBuf)
			responseLen = len(responseBuf)
			responseLenBuf = make([]byte, 2)
			responseLenBuf[0] = byte(responseLen / 256)
			responseLenBuf[1] = byte(responseLen % 256)
		} else {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "let forwarder handle domain \"%s\"", domainName)
			doForward = true
		}
	}
	// If queried domain is not black listed, forward the query to forwarder.
	fmt.Println("Do forward?", doForward)
	if doForward {
		fmt.Println("Do forward", dnsd.TCPForwardTo)
		myForwarder, err := net.Dial("tcp", dnsd.TCPForwardTo)
		fmt.Println("Forwarder error is", err)
		if err != nil {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error")
			return
		}
		//defer myForwarder.Close()
		// Send original query to forwarder without modification
		//myForwarder.SetDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		combinedBuf := make([]byte, len(queryLenBuf)+len(queryBuf))
		fmt.Println("Query length is ", queryLen)
		copy(combinedBuf[0:2], queryLenBuf)
		fmt.Println("Query len buf is", queryLenBuf)
		copy(combinedBuf[2:], queryBuf)
		fmt.Println("Query buf is", queryBuf)
		fmt.Println("Combined buf is", combinedBuf)

		if _, err = myForwarder.Write(combinedBuf); err != nil {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error")
			return
		} /*else if _, err = myForwarder.Write(queryBuf); err != nil {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error")
			return
		}*/
		// Retrieve forwarder's response
		myForwarder.Close()
		responseLenBuf = make([]byte, 2)
		if _, err = myForwarder.Read(responseLenBuf); err != nil {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error")
			return
		}
		responseLen = int(responseLenBuf[0])*256 + int(responseLenBuf[1])
		if responseLen > MaxPacketSize || responseLen < 1 {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "bad response length from forwarder")
			return
		}
		responseBuf = make([]byte, responseLen)
		if _, err = myForwarder.Read(responseBuf); err != nil {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error")
			return
		}
	}
	// Send response to my client
	fmt.Println("Response is", responseLenBuf, responseBuf)
	if _, err = clientConn.Write(responseLenBuf); err != nil {
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error")
		return
	} else if _, err = clientConn.Write(responseBuf); err != nil {
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error")
		return
	}
	return
}

/*
You may call this function only after having called Initialise()!
Start DNS daemon to listen on TCP port only. Block caller.
*/
func (dnsd *DNSD) StartAndBlockTCP() error {
	listenAddr := fmt.Sprintf("%s:%d", dnsd.TCPListenAddress, dnsd.TCPListenPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	dnsd.Logger.Printf("StartAndBlockTCP", listenAddr, nil, "going to listen for queries")
	for {
		clientConn, err := listener.Accept()
		if err != nil {
			return err
		}
		go dnsd.HandleTCPQuery(clientConn)
	}

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
	// Start both UDP and TCP listeners
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
