package dnsd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

const (
	RateLimitIntervalSec        = 1         // Rate limit is calculated at 1 second interval
	ForwarderTimeoutSec         = 1 * 2     // ForwarderTimeoutSec is the IO timeout for a round trip interaction with forwarders
	ClientTimeoutSec            = 30 * 2    // AnswerTimeoutSec is the IO timeout for a round trip interaction with DNS clients
	MaxPacketSize               = 9038      // Maximum acceptable UDP packet size
	BlacklistUpdateIntervalSec  = 12 * 3600 // Update ad-server blacklist at this interval
	BlacklistInitialDelaySec    = 120       // BlacklistInitialDelaySec is the number of seconds to wait for downloading blacklists for the first time.
	MinNameQuerySize            = 14        // If a query packet is shorter than this length, it cannot possibly be a name query.
	PublicIPRefreshIntervalSec  = 900       // PublicIPRefreshIntervalSec is how often the program places its latest public IP address into array of IPs that may query the server.
	BlackListDownloadTimeoutSec = 30        // BlackListDownloadTimeoutSec is the timeout to use when downloading blacklist hosts files.
	BlacklistMaxEntries         = 100000    // BlackListMaxEntries is the maximum number of entries to be accepted into black list after retireving them from public sources.
	TextCommandReplyTTL         = 30        // TextCommandReplyTTL is the TTL of text command reply, in number of seconds. Leave it low.
	/*
		ToolboxCommandPrefix is a short string that indicates a TXT query is most likely toolbox command. Keep it short,
		as DNS query input has to be pretty short.
	*/
	ToolboxCommandPrefix = '_'
)

/*
DefaultForwarders is a list of well tested, public, recursive DNS resolvers that must support both TCP and UDP for
queries. When DNS daemon's forwarders are left unspecified, it will use these default forwarders.

All of the resolvers below claim to improve cypher security to some degree.
*/
var DefaultForwarders = []string{
	// Quad9 (https://www.quad9.net)
	"9.9.9.9:53",
	"149.112.112.112:53",
	// SafeDNS (https://www.safedns.com)
	"195.46.39.39:53",
	"195.46.39.40:53",
	// OpenDNS (https://www.opendns.com/setupguide/)
	"208.67.222.222:53",
	"208.67.220.220:53",
	// Do not use Comodo SecureDNS because it has severe reliability issue as of 2018-03-30
	// Do not use neustar based resolvers (neustar.biz, norton connectsafe, etc) as they are teamed up with yahoo search
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
	Address              string                   `json:"Address"`              // Network address for both TCP and UDP to listen to, e.g. 0.0.0.0 for all network interfaces.
	AllowQueryIPPrefixes []string                 `json:"AllowQueryIPPrefixes"` // AllowQueryIPPrefixes are the string prefixes in IPv4 and IPv6 client addresses that are allowed to query the DNS server.
	PerIPLimit           int                      `json:"PerIPLimit"`           // PerIPLimit is approximately how many concurrent users are expected to be using the server from same IP address
	Forwarders           []string                 `json:"Forwarders"`           // DefaultForwarders are recursive DNS resolvers that will resolve name queries. They must support both TCP and UDP.
	Processor            *common.CommandProcessor `json:"-"`                    // Processor enables TXT queries to execute toolbox command

	UDPPort int `json:"UDPPort"` // UDP port to listen on
	TCPPort int `json:"TCPPort"` // TCP port to listen on

	tcpListener net.Listener // Once TCP daemon is started, this is its listener.
	udpListener *net.UDPConn // Once UDP daemon is started, this is its listener.

	/*
		blackList is a map of domain names (in lower case) and their resolved IP addresses that should be blocked. In
		the context of DNS, queries made against the domain names will be answered 0.0.0.0 (black hole).
		The DNS daemon itself isn't too concerned with the IP address, however, this black list serves as a valuable
		input for blocking IP address access in sockd.
	*/
	blackList         map[string]struct{}
	blackListUpdating int32 // blackListUpdating is set to 1 when black list is being updated, and 0 otherwise.

	myPublicIP           string          // myPublicIP is the latest public IP address of the laitos server.
	blackListMutex       *sync.RWMutex   // Protect against concurrent access to black list
	allowQueryMutex      *sync.Mutex     // allowQueryMutex guards against concurrent access to AllowQueryIPPrefixes.
	allowQueryLastUpdate int64           // allowQueryLastUpdate is the Unix timestamp of the very latest automatic placement of computer's public IP into the array of AllowQueryIPPrefixes.
	rateLimit            *misc.RateLimit // Rate limit counter
	logger               lalog.Logger

	// latestCommandTimestamp is the unix timestamp recorded when the latest command was about to be executed.
	latestCommandTimestamp int64
	// latestCommandTimestamp is the content of the latest toolbox command input.
	latestCommandInput string
	// latestCommandTimestamp is the content of the latest toolbox command output.
	latestCommandOutput *toolbox.Result

	// processQueryTestCaseFunc works along side DNS query processing routine, it offers queried name to test case for inspection.
	processQueryTestCaseFunc func(string)
}

// Check configuration and initialise internal states.
func (daemon *Daemon) Initialise() error {
	if daemon.Address == "" {
		daemon.Address = "0.0.0.0"
	}
	if daemon.UDPPort < 1 && daemon.TCPPort < 1 {
		/*
			If any port is left at 0, the DNS daemon will not listen for that protocol. But if both are at 0, then
			by default listen for both protocols.
		*/
		daemon.TCPPort = 53
		daemon.UDPPort = 53
	}
	if daemon.PerIPLimit < 1 {
		daemon.PerIPLimit = 48 // reasonable for a network of 3 users
	}
	if daemon.Forwarders == nil || len(daemon.Forwarders) == 0 {
		daemon.Forwarders = make([]string, len(DefaultForwarders))
		copy(daemon.Forwarders, DefaultForwarders)
	}
	daemon.logger = lalog.Logger{
		ComponentName: "dnsd",
		ComponentID:   []lalog.LoggerIDField{{Key: "TCP", Value: daemon.TCPPort}, {Key: "UDP", Value: daemon.UDPPort}},
	}
	if daemon.Processor == nil || daemon.Processor.IsEmpty() {
		daemon.logger.Info("Initialise", "", nil, "daemon will not be able to execute toolbox commands due to lack of command processor filter configuration")
		daemon.Processor = common.GetEmptyCommandProcessor()
	}
	daemon.Processor.SetLogger(daemon.logger)
	if errs := daemon.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("dnsd.Initialise: %+v", errs)
	}
	if daemon.AllowQueryIPPrefixes == nil {
		daemon.AllowQueryIPPrefixes = []string{}
	}
	for _, prefix := range daemon.AllowQueryIPPrefixes {
		if prefix == "" {
			return errors.New("DNSD.Initialise: IP address prefixes that are allowed to query may not contain empty string")
		}
	}

	daemon.allowQueryMutex = new(sync.Mutex)
	daemon.blackListMutex = new(sync.RWMutex)
	daemon.blackList = make(map[string]struct{})

	daemon.rateLimit = &misc.RateLimit{
		MaxCount: daemon.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
		Logger:   daemon.logger,
	}
	daemon.rateLimit.Initialise()

	// Always allow server itself to query the DNS servers via its public IP
	daemon.allowMyPublicIP()
	return nil
}

// allowMyPublicIP refreshes the public IP address of the DNS server, so that Internet clients that use laitos server as VPN server may use it for DNS as well.
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
		daemon.logger.Warning("allowMyPublicIP", "", nil, "unable to determine public IP address, the computer will not be able to send query to itself.")
		return
	}
	daemon.myPublicIP = latestIP
	daemon.logger.Info("allowMyPublicIP", "", nil, "the latest public IP address %s of this computer is now allowed to query", daemon.myPublicIP)
}

// checkAllowClientIP returns true only if the input IP address is among the allowed addresses.
func (daemon *Daemon) checkAllowClientIP(clientIP string) bool {
	if clientIP == "" || len(clientIP) > 64 {
		return false
	}
	// Fast track - always allow localhost to query
	if strings.HasPrefix(clientIP, "127.") || clientIP == "::1" || clientIP == daemon.myPublicIP {
		return true
	}
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

/*
repeatLastCommandOutput uses system clock and the input command to determine whether the request should be served with
the output from latest command. If the input command matches the latest command input and the TTL of command reply
has not expired, then the latest command output will be returned; otherwise, nil will be returned.
*/
func (daemon *Daemon) repeatLastCommandOutput(thisCommandInput string) (previousOutput *toolbox.Result) {
	if time.Now().Before(time.Unix(daemon.latestCommandTimestamp+TextCommandReplyTTL, 0)) && thisCommandInput == daemon.latestCommandInput {
		return daemon.latestCommandOutput
	}
	return nil
}

/*
UpdateBlackList downloads the latest blacklist files from PGL and MVPS, resolves the IP addresses of each domain,
and stores the latest blacklist names and IP addresses into blacklist map.
*/
func (daemon *Daemon) UpdateBlackList(maxEntries int) {
	if !atomic.CompareAndSwapInt32(&daemon.blackListUpdating, 0, 1) {
		daemon.logger.Info("UpdateBlackList", "", nil, "will skip this run because update routine is already ongoing")
		return
	}
	defer func() {
		atomic.StoreInt32(&daemon.blackListUpdating, 0)
	}()

	// Download black list data from all sources
	allNames := DownloadAllBlacklists(daemon.logger)
	if len(allNames) > maxEntries {
		allNames = allNames[:maxEntries]
	}
	// Get ready to construct the new blacklist
	newBlackList := make(map[string]struct{})
	newBlackListMutex := new(sync.Mutex)
	numRoutines := 16
	if misc.HostIsWindows() {
		/*
			Windows is very slow to do concurrent DNS lookup, these parallel routines will even trick windows into
			thinking that there is no Internet anymore. Pretty weird.
		*/
		numRoutines = 4
	}
	parallelResolve := new(sync.WaitGroup)
	parallelResolve.Add(numRoutines)
	// Collect some nice counter data just for show
	var countResolvedNames, countNonResolvableNames, countResolvedIPs, countResolutionAttempts int64
	for i := 0; i < numRoutines; i++ {
		go func(i int) {
			defer parallelResolve.Done()
			for j := i * (len(allNames) / numRoutines); j < (i+1)*(len(allNames)/numRoutines); j++ {
				// Count number of resolution attempts only for logging the progress
				atomic.AddInt64(&countResolutionAttempts, 1)
				if atomic.LoadInt64(&countResolutionAttempts)%500 == 1 {
					daemon.logger.Info("UpdateBlackList", "", nil, "resolving %d of %d black listed domain names",
						atomic.LoadInt64(&countResolutionAttempts), len(allNames))
				}
				name := strings.ToLower(strings.TrimSpace(allNames[j]))
				ips, err := net.LookupIP(name)
				newBlackListMutex.Lock()
				newBlackList[name] = struct{}{}
				if err == nil {
					atomic.AddInt64(&countResolvedNames, 1)
					atomic.AddInt64(&countResolvedIPs, int64(len(ips)))
					for _, ip := range ips {
						newBlackList[ip.String()] = struct{}{}
					}
				} else {
					atomic.AddInt64(&countNonResolvableNames, 1)
				}
				newBlackListMutex.Unlock()
			}
		}(i)
	}
	parallelResolve.Wait()
	// Use the newly constructed blacklist from now on
	daemon.blackListMutex.Lock()
	daemon.blackList = newBlackList
	daemon.blackListMutex.Unlock()
	daemon.logger.Info("UpdateBlackList", "", nil, "out of %d domains, %d are successfully resolved into %d IPs, %d failed, and now blacklist has %d entries",
		len(allNames), countResolvedNames, countResolvedIPs, countNonResolvableNames, len(newBlackList))
}

/*
You may call this function only after having called Initialise()!
Start DNS daemon on configured TCP and UDP ports. Block caller until both listeners are told to stop.
If either TCP or UDP port fails to listen, all listeners are closed and an error is returned.
*/
func (daemon *Daemon) StartAndBlock() error {
	// Update ad-block black list in background
	stopAdBlockUpdater := make(chan bool, 2)
	go func() {
		firstTime := true
		nextRunAt := time.Now().Add(BlacklistInitialDelaySec * time.Second)
		for {
			if firstTime {
				select {
				case <-stopAdBlockUpdater:
					return
				case <-time.After(time.Until(nextRunAt)):
					nextRunAt = nextRunAt.Add(BlacklistUpdateIntervalSec * time.Second)
					daemon.UpdateBlackList(BlacklistMaxEntries)
				}
				firstTime = false
			} else {
				// Afterwards, try to maintain a steady rate of execution.
				select {
				case <-stopAdBlockUpdater:
					return
				case <-time.After(time.Until(nextRunAt)):
					nextRunAt = nextRunAt.Add(time.Duration(BlacklistUpdateIntervalSec) * time.Second)
					daemon.UpdateBlackList(BlacklistMaxEntries)
				}
			}
		}
	}()

	// Start server listeners
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
			daemon.logger.Warning("Stop", "", err, "failed to close TCP listener")
		}
	}
	if listener := daemon.udpListener; listener != nil {
		if err := listener.Close(); err != nil {
			daemon.logger.Warning("Stop", "", err, "failed to close UDP listener")
		}
	}
}

/*
IsInBlacklist returns true only if the input domain name or IP address is black listed. If the domain name represents
a sub-domain name, then the function strips the sub-domain portion in order to check it against black list.
*/
func (daemon *Daemon) IsInBlacklist(nameOrIP string) bool {
	daemon.blackListMutex.RLock()
	defer daemon.blackListMutex.RUnlock()
	// If the name is exceedingly long, then return true as if the name is black-listed.
	if len(nameOrIP) > 255 {
		return true
	}
	// Black list only contains lower case names, hence converting the input name to lower case for matching.
	nameOrIP = strings.ToLower(strings.TrimSpace(nameOrIP))
	/*
		Starting from the requested domain name, strip down sub-domain name to make candidates for black list match.
		Stripping down an IP address is meaningless but will do no harm.
	*/
	blackListCandidates := make([]string, 0, 4)
	blackListCandidates = append(blackListCandidates, nameOrIP)
	for {
		// Remove sub-domain name prefix
		index := strings.IndexRune(nameOrIP, '.')
		if index < 1 || index == len(nameOrIP)-1 {
			break
		}
		nameOrIP = nameOrIP[index+1:]
		if len(nameOrIP) < 4 {
			// It is impossible to have a domain name shorter than 4 characters, therefore stop further stripping.
			continue
		}
		blackListCandidates = append(blackListCandidates, nameOrIP)
	}
	// Check each broken-down variation of domain name against black list
	for _, candidate := range blackListCandidates {
		if _, blacklisted := daemon.blackList[candidate]; blacklisted {
			return true
		}
	}
	return false
}

// nameQueryMagic is a series of bytes that appears in a DNS name (A) query.
var nameQueryMagic = []byte{0, 1, 0, 1}

// textQueryMagic is a series of bytes that appears in a DNS text query.
var textQueryMagic = []byte{0, 16, 0, 1}

// isTextQuery returns true only if the input query appears to be a text query.
func isTextQuery(queryBody []byte) bool {
	typeTXTClassIN := bytes.Index(queryBody[13:], textQueryMagic)
	return typeTXTClassIN > 0
}

/*
testResolveNameAndBlackList is a common test case that tests name resolution of popular domain names as well as black
list domain names.
*/
func testResolveNameAndBlackList(t testingstub.T, daemon *Daemon, resolver *net.Resolver) {
	// Track and verify the last resolved name
	var lastResolvedName string
	daemon.processQueryTestCaseFunc = func(queryInput string) {
		lastResolvedName = queryInput
	}

	// Resolve A and TXT records from popular domains
	for _, domain := range []string{"apple.com", "bing.com", "github.com"} {
		lastResolvedName = ""
		if result, err := resolver.LookupTXT(context.Background(), domain); err != nil || len(result) == 0 || len(result[0]) == 0 {
			t.Fatal("failed to resolve domain name TXT record", domain, err, result)
		}
		if lastResolvedName != domain {
			t.Fatal("attempted to resolve", domain, "but daemon saw:", lastResolvedName)
		}
		lastResolvedName = ""
		if result, err := resolver.LookupHost(context.Background(), domain); err != nil || len(result) == 0 || len(result[0]) == 0 {
			t.Fatal("failed to resolve domain name A record", domain, err, result)
		}
		if lastResolvedName != domain {
			t.Fatal("attempted to resolve", domain, "but daemon saw:", lastResolvedName)
		}
	}

	// Blacklist github and see if query gets a black hole response
	oldBlacklist := daemon.blackList
	defer func() {
		daemon.blackList = oldBlacklist
	}()
	daemon.blackList["github.com"] = struct{}{}
	if result, err := resolver.LookupHost(context.Background(), "GiThUb.CoM"); err != nil || len(result) != 1 || result[0] != "0.0.0.0" {
		t.Fatal("failed to get a black-listed response", err, result)
	}
	if lastResolvedName != "GiThUb.CoM" {
		t.Fatal("attempted to resolve black-listed github.com, but daemon saw:", lastResolvedName)
	}

	// Make a TXT query that carries toolbox command prefix but is in fact not
	if result, err := resolver.LookupTXT(context.Background(), "_.apple.com"); err != nil || len(result) == 0 || len(result[0]) == 0 {
		// _.apple.com.            3599    IN      TXT     "v=spf1 redirect=_spf.apple.com"
		t.Fatal(result, err)
	}

	// Make a TXT query that carries toolbox command prefix and an invalid PIN
	if result, err := resolver.LookupTXT(context.Background(), "_badpin .s echo hi"); err == nil || result != nil {
		t.Fatal(result, err)
	}

	// Prefix _ indicates it is a toolbox command, DTMF sequence 142 becomes a full-stop, 0 becomes a space.
	var cmdInput = "_verysecret142s0date"
	thisYear := strconv.Itoa(time.Now().Year())
	// Make a TXT query that carries toolbox command prefix and a valid command
	result, err := resolver.LookupTXT(context.Background(), cmdInput+".example.com")
	if err != nil || len(result) == 0 || !strings.Contains(result[0], thisYear) {
		t.Fatal(result, err)
	}
	// Rapidly making the same request should receive the same command response
	if repeatResult, err := resolver.LookupTXT(context.Background(), cmdInput+".example.com"); err != nil || !reflect.DeepEqual(repeatResult, result) {
		t.Fatal(repeatResult, result, err)
	}
	// Wait for TTL to expire and repeat the same request, it should receive a new response.
	time.Sleep((TextCommandReplyTTL + 1) * time.Second)
	if repeatResult, err := resolver.LookupTXT(context.Background(), cmdInput+".example.com"); err != nil || reflect.DeepEqual(repeatResult, result) || !strings.Contains(result[0], thisYear) {
		t.Fatal(repeatResult, result, err)
	}
}
