package dnsd

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	RateLimitIntervalSec       = 1    // Rate limit is calculated at 1 second interval
	IOTimeoutSec               = 60   // IO timeout for both read and write operations
	MaxPacketSize              = 9038 // Maximum acceptable UDP packet size
	NumQueueRatio              = 10   // Upon initialisation, create (PerIPLimit/NumQueueRatio) number of queues to handle queries.
	BlacklistUpdateIntervalSec = 7200 // Update ad-server blacklist at this interval
	MinNameQuerySize           = 14   // If a query packet is shorter than this length, it cannot possibly be a name query.
	PublicIPRefreshIntervalSec = 900  // PublicIPRefreshIntervalSec is how often the program places its latest public IP address into array of IPs that may query the server.
	MVPSLicense                = `Disclaimer: this file is free to use for personal use only. Furthermore it is NOT permitted to ` +
		`copy any of the contents or host on any other site without permission or meeting the full criteria of the below license ` +
		` terms. This work is licensed under the Creative Commons Attribution-NonCommercial-ShareAlike License. ` +
		` http://creativecommons.org/licenses/by-nc-sa/4.0/ License info for commercial purposes contact Winhelp2002`
)

/*
Forwarders is a list of well tested, public, recursive DNS resolvers that must support both TCP and UDP for queries.
All of the resolvers below claim to improve cypher security to some degree.
*/
var Forwarders = []string{
	// Comodo SecureDNS (https://www.comodo.com/secure-dns/)
	"8.26.56.26:53",
	"8.20.247.20:53",
	// Neustar free recursive DNS with threat protection (https://www.neustar.biz/security/dns-services/free-recursive-dns-service)
	"156.154.70.2:53",
	"156.154.71.2:53",
	// Quad9 (https://www.quad9.net)
	"9.9.9.9:53",
	"149.112.112.112:53",
	// Norton ConnectSafe A (https://dns.norton.com/configureRouter.html)
	"199.85.126.10:53",
	"199.85.127.10:53", // 2017-11-27 - this alternative address is unstable in answering TCP requests, though UDP is fine.
}

// A query to forward to DNS forwarder via DNS.
type UDPQuery struct {
	MyServer    *net.UDPConn
	ClientAddr  *net.UDPAddr
	QueryPacket []byte
}

// A query to forward to DNS forwarder via TCP.
type TCPForwarderQuery struct {
	MyServer    *net.Conn
	QueryPacket []byte
}

// A DNS forwarder daemon that selectively refuse to answer certain A record requests made against advertisement servers.
type Daemon struct {
	Address              string   `json:"Address"`              // Network address for both TCP and UDP to listen to, e.g. 0.0.0.0 for all network interfaces.
	AllowQueryIPPrefixes []string `json:"AllowQueryIPPrefixes"` // AllowQueryIPPrefixes are the string prefixes in IPv4 and IPv6 client addresses that are allowed to query the DNS server.
	PerIPLimit           int      `json:"PerIPLimit"`           // PerIPLimit is approximately how many concurrent users are expected to be using the server from same IP address

	UDPPort int `json:"UDPPort"` // UDP port to listen on
	TCPPort int `json:"TCPPort"` // TCP port to listen on

	tcpListener       net.Listener     // Once TCP daemon is started, this is its listener.
	udpForwardConn    []net.Conn       // UDP connections made toward forwarder
	udpForwarderQueue []chan *UDPQuery // Processing queues that handle UDP forward queries
	udpBlackHoleQueue []chan *UDPQuery // Processing queues that handle UDP black-list answers
	udpListener       *net.UDPConn     // Once UDP daemon is started, this is its listener.

	/*
		blackList is a map of domain names (in lower case) and their resolved IP addresses that should be blocked. In
		the context of DNS, queries made against the domain names will be answered 0.0.0.0 (black hole).
		The DNS daemon itself isn't too concerned with the IP address, however, this black list serves as a valuable
		input for blocking IP address access in sockd.
	*/
	blackList map[string]struct{}

	blackListMutex       *sync.RWMutex   // Protect against concurrent access to black list
	allowQueryMutex      *sync.Mutex     // allowQueryMutex guards against concurrent access to AllowQueryIPPrefixes.
	allowQueryLastUpdate int64           // allowQueryLastUpdate is the Unix timestamp of the very latest automatic placement of computer's public IP into the array of AllowQueryIPPrefixes.
	rateLimit            *misc.RateLimit // Rate limit counter
	logger               misc.Logger
}

// Check configuration and initialise internal states.
func (daemon *Daemon) Initialise() error {
	daemon.logger = misc.Logger{ComponentName: "DNSD", ComponentID: fmt.Sprintf("%s-%d&%d", daemon.Address, daemon.TCPPort, daemon.UDPPort)}
	if daemon.Address == "" {
		return errors.New("DNSD.Initialise: listen address must not be empty")
	}
	if daemon.UDPPort < 1 && daemon.TCPPort < 1 {
		return errors.New("DNSD.Initialise: either or both TCP and UDP ports must be specified and be greater than 0")
	}
	if daemon.PerIPLimit < 1 {
		return errors.New("DNSD.Initialise: PerIPLimit must be greater than 0")
	}
	if len(daemon.AllowQueryIPPrefixes) == 0 {
		return errors.New("DNSD.Initialise: allowable IP prefixes list must not be empty")
	}
	for _, prefix := range daemon.AllowQueryIPPrefixes {
		if prefix == "" {
			return errors.New("DNSD.Initialise: any allowable IP prefixes must not be empty string")
		}
	}
	// Always allow localhost to query via both IPv4 and IPv6
	daemon.AllowQueryIPPrefixes = append(daemon.AllowQueryIPPrefixes, "127.", "::1")

	daemon.allowQueryMutex = new(sync.Mutex)
	daemon.blackListMutex = new(sync.RWMutex)
	daemon.blackList = make(map[string]struct{})

	daemon.rateLimit = &misc.RateLimit{
		MaxCount: daemon.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
		Logger:   daemon.logger,
	}
	daemon.rateLimit.Initialise()
	// Create a number of forwarder queues to handle incoming UDP DNS queries
	// Keep in mind, TCP queries are not handled by queues.
	if daemon.UDPPort > 0 {
		numQueues := daemon.PerIPLimit / NumQueueRatio
		// At very least, each forwarder address has to get a queue.
		if numQueues < len(Forwarders) {
			numQueues = len(Forwarders)
		}
		daemon.udpForwardConn = make([]net.Conn, numQueues)
		daemon.udpForwarderQueue = make([]chan *UDPQuery, numQueues)
		daemon.udpBlackHoleQueue = make([]chan *UDPQuery, numQueues)
		for i := 0; i < numQueues; i++ {
			/*
				Each queue is connected to a different forwarder.
				When a DNS query comes in, it is assigned a random forwarder to be processed.
			*/
			forwarderAddr, err := net.ResolveUDPAddr("udp", Forwarders[i%len(Forwarders)])
			if err != nil {
				return fmt.Errorf("DNSD.Initialise: failed to resolve UDP address - %v", err)
			}
			forwarderConn, err := net.DialTimeout("udp", forwarderAddr.String(), IOTimeoutSec*time.Second)
			if err != nil {
				return fmt.Errorf("DNSD.Initialise: failed to connect to UDP forwarder - %v", err)
			}
			daemon.udpForwardConn[i] = forwarderConn
			daemon.udpForwarderQueue[i] = make(chan *UDPQuery, 16) // there really is no need for a deeper queue
			daemon.udpBlackHoleQueue[i] = make(chan *UDPQuery, 4)  // there is also no need for a deeper queue here
		}
	}

	// Always allow server to query itself via public IP
	daemon.allowMyPublicIP()
	return nil
}

// allowMyPublicIP places the computer's public IP address into the array of IPs allowed to query the server.
func (daemon *Daemon) allowMyPublicIP() {
	if daemon.allowQueryLastUpdate+PublicIPRefreshIntervalSec >= time.Now().Unix() {
		return
	}
	daemon.allowQueryMutex.Lock()
	defer daemon.allowQueryMutex.Unlock()
	defer func() {
		// This routine runs periodically no matter it succeeded or failed in retrieving latest public IP
		daemon.allowQueryLastUpdate = time.Now().Unix()
	}()
	latestIP := inet.GetPublicIP()
	if latestIP == "" {
		// Not a fatal error if IP cannot be determined
		daemon.logger.Warningf("allowMyPublicIP", "", nil, "unable to determine public IP address, the computer will not be able to send query to itself.")
		return
	}
	foundMyIP := false
	for _, allowedIP := range daemon.AllowQueryIPPrefixes {
		if allowedIP == latestIP {
			foundMyIP = true
			break
		}
	}
	if !foundMyIP {
		// Place latest IP into the array, but do not erase the old IP entries.
		daemon.AllowQueryIPPrefixes = append(daemon.AllowQueryIPPrefixes, latestIP)
		daemon.logger.Printf("allowMyPublicIP", "", nil, "the latest public IP address %s of this computer is now allowed to query", latestIP)
	}
}

// checkAllowClientIP returns true only if the input IP address is among the allowed addresses.
func (daemon *Daemon) checkAllowClientIP(clientIP string) bool {
	// At regular time interval, make sure that the latest public IP is allowed to query.
	daemon.allowMyPublicIP()

	daemon.allowQueryMutex.Lock()
	defer daemon.allowQueryMutex.Unlock()
	for _, prefix := range daemon.AllowQueryIPPrefixes {
		if strings.HasPrefix(clientIP, prefix) {
			return true
		}
	}
	return false
}

// Download ad-servers list from pgl.yoyo.org and return those domain names.
func (daemon *Daemon) GetAdBlacklistPGL() ([]string, error) {
	yoyo := "https://pgl.yoyo.org/adservers/serverlist.php?hostformat=nohtml&showintro=0&mimetype=plaintext"
	resp, err := inet.DoHTTP(inet.HTTPRequest{TimeoutSec: 30}, yoyo)
	if err != nil {
		return []string{}, err
	}
	if statusErr := resp.Non2xxToError(); statusErr != nil {
		return []string{}, statusErr
	}
	lines := strings.Split(string(resp.Body), "\n")
	if len(lines) < 100 {
		return []string{}, fmt.Errorf("DNSD.GetAdBlacklistPGL: PGL's ad-server list is suspiciously short at only %d lines", len(lines))
	}
	names := make([]string, 0, len(lines))
	for _, line := range lines {
		names = append(names, strings.ToLower(strings.TrimSpace(line)))
	}
	return names, nil
}

// Download ad-servers list from winhelp2002.mvps.org and return those domain names.
func (daemon *Daemon) GetAdBlacklistMVPS() ([]string, error) {
	yoyo := "http://winhelp2002.mvps.org/hosts.txt"
	resp, err := inet.DoHTTP(inet.HTTPRequest{TimeoutSec: 30}, yoyo)
	if err != nil {
		return []string{}, err
	}
	if statusErr := resp.Non2xxToError(); statusErr != nil {
		return []string{}, statusErr
	}
	// Collect host names from the hosts file content
	names := make([]string, 0, 16384)
	for _, line := range strings.Split(string(resp.Body), "\n") {
		indexZero := strings.Index(line, "0.0.0.0")
		nameEnd := strings.IndexRune(line, '#')
		if indexZero == -1 {
			// Skip lines that do not have a host name
			continue
		}
		if nameEnd == -1 {
			nameEnd = len(line)
		}
		nameBegin := indexZero + len("0.0.0.0")
		if nameBegin >= nameEnd {
			// The line looks like # this is a comment 0.0.0.0
			continue
		}
		names = append(names, strings.ToLower(strings.TrimSpace(line[nameBegin:nameEnd])))
	}
	if len(names) < 100 {
		return []string{}, fmt.Errorf("DNSD.GetAdBlacklistMVPS: MVPS' ad-server list is suspiciously short at only %d lines", len(names))
	}
	return names, nil
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

/*
ExtractDomainName extracts domain name requested by input query packet. If the function fails to determine the requested
name, it returns an empty string.
*/
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
	// Convert the name to lower case because sometimes device queries names in mixed case
	domainName := strings.ToLower(strings.TrimSpace(string(domainNameBytes)))
	if len(domainName) > 1024 {
		// Domain name is unrealistically long
		return ""
	}
	return domainName
}

/*
UpdatedAdBlockLists downloads the latest blacklist files from PGL and MVPS, resolves the IP addresses of each domain,
and stores the latest blacklist names and IP addresses into blacklist map.
*/
func (daemon *Daemon) UpdatedAdBlockLists() {
	pglEntries, pglErr := daemon.GetAdBlacklistPGL()
	if pglErr == nil {
		daemon.logger.Printf("GetAdBlacklistPGL", "", nil, "successfully retrieved ad-blacklist with %d entries", len(pglEntries))
	} else {
		daemon.logger.Warningf("GetAdBlacklistPGL", "", pglErr, "failed to update ad-blacklist")
	}
	mvpsEntries, mvpsErr := daemon.GetAdBlacklistMVPS()
	if mvpsErr == nil {
		daemon.logger.Printf("GetAdBlacklistMVPS", "", nil, "successfully retrieved ad-blacklist with %d entries", len(mvpsEntries))
		daemon.logger.Printf("GetAdBlacklistMVPS", "", nil, "Please comply with the following license for your usage of http://winhelp2002.mvps.org/hosts.txt: %s", MVPSLicense)
	} else {
		daemon.logger.Warningf("GetAdBlacklistMVPS", "", mvpsErr, "failed to update ad-blacklist")
	}
	// Put names from both lists into one list
	allEntries := make([]string, len(pglEntries)+len(mvpsEntries))
	copy(allEntries, pglEntries)
	copy(allEntries[len(pglEntries):], mvpsEntries)
	// Get ready to construct the new blacklist
	newBlackList := make(map[string]struct{})
	newBlackListMutex := new(sync.Mutex)
	// Use a parallel approach to resolve these names
	numRoutines := 16
	parallelResolve := new(sync.WaitGroup)
	parallelResolve.Add(numRoutines)
	// Collect some nice counter data just for show
	var countResolvedNames, countNonResolvableNames, countResolvedIPs int64
	for i := 0; i < numRoutines; i++ {
		go func(i int) {
			for j := i * (len(allEntries) / numRoutines); j < (i+1)*(len(allEntries)/numRoutines); j++ {
				if j%500 == 1 {
					daemon.logger.Printf("UpdatedAdBlockLists", "", nil, "resolving IP addresses at index %d", j)
				}
				name := strings.ToLower(strings.TrimSpace(allEntries[j]))
				// In case MVPS or PGL gets mad at me and returns "com" or "net" to block, for a prank :>
				if len(name) < 5 {
					continue
				}
				ips, err := net.LookupIP(name)
				newBlackListMutex.Lock()
				if err == nil {
					atomic.AddInt64(&countResolvedNames, 1)
					atomic.AddInt64(&countResolvedIPs, int64(len(ips)))
					newBlackList[name] = struct{}{}
					for _, ip := range ips {
						newBlackList[ip.String()] = struct{}{}
					}
				} else {
					atomic.AddInt64(&countNonResolvableNames, 1)
				}
				newBlackListMutex.Unlock()
			}
			parallelResolve.Done()
		}(i)
	}
	parallelResolve.Wait()
	// Use the newly constructed blacklist from now on
	daemon.blackListMutex.Lock()
	daemon.blackList = newBlackList
	daemon.blackListMutex.Unlock()
	daemon.logger.Printf("UpdatedAdBlockLists", "", nil, "out of %d domains, %d are successfully resolved into %d IPs, %d failed, and now blacklist has %d entries",
		len(allEntries), countResolvedNames, countResolvedIPs, countNonResolvableNames, len(newBlackList))
}

/*
You may call this function only after having called Initialise()!
Start DNS daemon on configured TCP and UDP ports. Block caller until both listeners are told to stop.
If either TCP or UDP port fails to listen, all listeners are closed and an error is returned.
*/
func (daemon *Daemon) StartAndBlock() error {
	// Keep updating ad-block black list in background
	stopAdBlockUpdater := make(chan bool, 1)
	go func() {
		daemon.UpdatedAdBlockLists()
		for {
			select {
			case <-stopAdBlockUpdater:
				return
			case <-time.After(BlacklistUpdateIntervalSec * time.Second):
				daemon.UpdatedAdBlockLists()
			}
		}
	}()
	numListeners := 0
	errChan := make(chan error, 2)
	if daemon.UDPPort != 0 {
		numListeners++
		go func() {
			err := daemon.StartAndBlockUDP()
			errChan <- err
			stopAdBlockUpdater <- true
		}()
	}
	if daemon.TCPPort != 0 {
		numListeners++
		go func() {
			err := daemon.StartAndBlockTCP()
			errChan <- err
			stopAdBlockUpdater <- true
		}()
	}
	for i := 0; i < numListeners; i++ {
		if err := <-errChan; err != nil {
			daemon.Stop()
			return err
		}
	}
	return nil
}

// Close all of open TCP and UDP listeners so that they will cease processing incoming connections.
func (daemon *Daemon) Stop() {
	if listener := daemon.tcpListener; listener != nil {
		if err := listener.Close(); err != nil {
			daemon.logger.Warningf("Stop", "", err, "failed to close TCP listener")
		}
	}
	if listener := daemon.udpListener; listener != nil {
		if err := listener.Close(); err != nil {
			daemon.logger.Warningf("Stop", "", err, "failed to close UDP listener")
		}
	}
}

/*
IsInBlacklist returns true if any of the input domain name or IPs is black listed. It will correctly identify sub-domain
names and verify them against blacklist as well.
*/
func (daemon *Daemon) IsInBlacklist(nameOrIPs ...string) bool {
	daemon.blackListMutex.RLock()
	defer daemon.blackListMutex.RUnlock()
	for _, name := range nameOrIPs {
		name = strings.ToLower(strings.TrimSpace(name))
		// In case name is a sub-domain, break it down. If name is an IP, the process won't do any harm.
		domainBreakdown := make([]string, 0, 4)
		// First name is simply the verbatim domain name as requested
		domainBreakdown = append(domainBreakdown, name)
		// Append more of the same domain name, each with leading component removed.
		for {
			index := strings.IndexRune(name, '.')
			if index < 1 || index == len(name)-1 {
				break
			}
			name = name[index+1:]
			if len(name) < 4 {
				// It is impossible to have a domain name shorter than 4 characters, therefore stop breaking down here.
				continue
			}
			domainBreakdown = append(domainBreakdown, name)
		}
		// Check each broken-down variation of domain name against black list
		for _, brokenDownName := range domainBreakdown {
			_, blacklisted := daemon.blackList[brokenDownName]
			if blacklisted {
				return true
			}
		}
	}
	return false
}

var githubComTCPQuery, githubComUDPQuery []byte // Sample queries for composing test cases

func init() {
	var err error
	// Prepare two A queries on "github.coM" (note the capital M, hex 4d) for test cases
	githubComTCPQuery, err = hex.DecodeString("00274cc7012000010000000000010667697468756203636f4d00000100010000291000000000000000")
	if err != nil {
		panic(err)
	}
	githubComUDPQuery, err = hex.DecodeString("e575012000010000000000010667697468756203636f4d00000100010000291000000000000000")
	if err != nil {
		panic(err)
	}
}
