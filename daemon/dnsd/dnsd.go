package dnsd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/platform"
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

const (
	RateLimitIntervalSec        = 1         // RateLimitIntervalSec is the interval (seconds) at which rate limit is measured and applied to DNS clients.
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

var (
	// DefaultForwarders is a list of well tested, public, recursive DNS resolvers that must support both TCP and UDP for queries.
	// When DNS daemon's forwarders are left unspecified, it will use these default forwarders.
	// Operators of the DNS resolvers below claim to offer enhanced cyber security to some degree.
	// Having more addresses in the list helps to improve DNS server reliability, as each client query is handled by a random forwarder.
	DefaultForwarders = []string{
		// Quad9 (https://www.quad9.net/)
		"9.9.9.9:53",
		"149.112.112.112:53",
		// CloudFlare with malware prevention (https://blog.cloudflare.com/introducing-1-1-1-1-for-families/)
		"1.1.1.2:53",
		"1.0.0.2:53",
		// OpenDNS (https://www.opendns.com/setupguide/)
		"208.67.222.222:53",
		"208.67.220.220:53",
		// AdGuard DNS (https://adguard.com/en/adguard-dns/overview.html)
		"94.140.14.14:53",
		"94.140.15.15:53",
		// Do not use SafeDNS (www.safedns.com) as it has severe reliability issue as of 2021-01-25.
		// Do not use Neustar (also known as "ultradns" and "dnsadvantage") as it often redirects users to their search home page,
		// sometimes maliciously (e.g. facebook -> search).
		// Do not use Comodo SecureDNS because it has severe reliability issue as of 2018-03-30.
		// Norton ConnectSafe was shut down in November 2018.
	}
)

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
	Address              string                    `json:"Address"`              // Network address for both TCP and UDP to listen to, e.g. 0.0.0.0 for all network interfaces.
	AllowQueryIPPrefixes []string                  `json:"AllowQueryIPPrefixes"` // AllowQueryIPPrefixes are the string prefixes in IPv4 and IPv6 client addresses that are allowed to query the DNS server.
	PerIPLimit           int                       `json:"PerIPLimit"`           // PerIPLimit is approximately how many concurrent users are expected to be using the server from same IP address
	Forwarders           []string                  `json:"Forwarders"`           // DefaultForwarders are recursive DNS resolvers that will resolve name queries. They must support both TCP and UDP.
	Processor            *toolbox.CommandProcessor `json:"-"`                    // Processor enables TXT queries to execute toolbox command

	UDPPort int `json:"UDPPort"` // UDP port to listen on
	TCPPort int `json:"TCPPort"` // TCP port to listen on

	tcpServer *common.TCPServer
	udpServer *common.UDPServer

	/*
		blackList is a map of domain names (in lower case) and their resolved IP addresses that should be blocked. In
		the context of DNS, queries made against the domain names will be answered 0.0.0.0 (black hole).
		The DNS daemon itself isn't too concerned with the IP address, however, this black list serves as a valuable
		input for blocking IP address access in sockd.
	*/
	blackList map[string]struct{}

	myPublicIP           string          // myPublicIP is the latest public IP address of the laitos server.
	blackListMutex       *sync.RWMutex   // Protect against concurrent access to black list
	allowQueryMutex      *sync.Mutex     // allowQueryMutex guards against concurrent access to AllowQueryIPPrefixes.
	allowQueryLastUpdate int64           // allowQueryLastUpdate is the Unix timestamp of the very latest automatic placement of computer's public IP into the array of AllowQueryIPPrefixes.
	rateLimit            *misc.RateLimit // Rate limit counter
	logger               lalog.Logger

	// latestCommands remembers the result of most recently executed toolbox commands.
	latestCommands *LatestCommands

	// processQueryTestCaseFunc works along side DNS query processing routine, it offers queried name to test case for inspection.
	processQueryTestCaseFunc func(string)
}

// Check configuration and initialise internal states.
func (daemon *Daemon) Initialise() error {
	if daemon.Address == "" {
		daemon.Address = "0.0.0.0"
	}
	if daemon.UDPPort < 1 && daemon.TCPPort < 1 {
		// If either port number is 0, then the DNS daemon will not serve that protocol.
		// If both port numbers are 0, then by default the daemon will serve both TCP and UDP clients.
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
		daemon.Processor = toolbox.GetEmptyCommandProcessor()
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

	daemon.latestCommands = NewLatestCommands()
	daemon.tcpServer = common.NewTCPServer(daemon.Address, daemon.TCPPort, "dnsd", daemon, daemon.PerIPLimit)
	daemon.udpServer = common.NewUDPServer(daemon.Address, daemon.UDPPort, "dnsd", daemon, daemon.PerIPLimit)

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
	daemon.logger.Info("allowMyPublicIP", "", nil, "the computer may send DNS queries to its public IP address %s", daemon.myPublicIP)
}

// checkAllowClientIP returns true only if the input client IP address is allowed to query this DNS server.
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
	// Another fast track - subjects that are monitored (periodically phoning home) are allowed to query the DNS server.
	// By convention, subject reports transmitted over IP network will have their client IP address recorded in report's client tag attribute.
	if daemon.Processor.Features.MessageProcessor.HasClientTag(clientIP) {
		return true
	}
	// Allow the client to query if the IP address matches any of the allowed address prefixes
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
UpdateBlackList downloads the latest blacklist files from PGL and MVPS, resolves the IP addresses of each domain,
and stores the latest blacklist names and IP addresses into blacklist map.
*/
func (daemon *Daemon) UpdateBlackList(blacklistedNames []string) {
	beginUnixSec := time.Now().Unix()
	// Get ready to construct the new blacklist
	newBlackList := make(map[string]struct{})
	newBlackListMutex := new(sync.Mutex)
	// Populate the list faster when getting started
	numRoutines := 12
	daemon.blackListMutex.RLock()
	if len(daemon.blackList) > 0 {
		// Slow down when updating the blacklist, there is no hurry. This helps to reduce DNS resolution load on the server host.
		numRoutines = 6
	}
	daemon.blackListMutex.RUnlock()
	if platform.HostIsWindows() {
		/*
			Windows is very slow to do concurrent DNS lookup, these parallel routines will even trick windows into
			thinking that there is no Internet anymore. Pretty weird.
		*/
		numRoutines /= 2
	}
	parallelResolve := new(sync.WaitGroup)
	parallelResolve.Add(numRoutines)
	// Collect some nice counter data just for show
	var countResolvedNames, countNonResolvableNames, countResolvedIPs, countResolutionAttempts int64
	// Use a well known public recursive DNS resolver, otherwise the DNS resolver of the LAN may be already blocking ads, leading
	for i := 0; i < numRoutines; i++ {
		go func(i int) {
			defer parallelResolve.Done()
			for j := i * (len(blacklistedNames) / numRoutines); j < (i+1)*(len(blacklistedNames)/numRoutines); j++ {
				// Count number of resolution attempts only for logging the progress
				atomic.AddInt64(&countResolutionAttempts, 1)
				if atomic.LoadInt64(&countResolutionAttempts)%500 == 1 {
					daemon.logger.Info("UpdateBlackList", "", nil, "resolving %d of %d black listed domain names",
						atomic.LoadInt64(&countResolutionAttempts), len(blacklistedNames))
				}
				name := strings.ToLower(strings.TrimSpace(blacklistedNames[j]))
				// The appearance of NULL byte triggers an unfortunate panic in go's DNS resolution routine on Windows alone
				if strings.ContainsRune(name, 0) {
					continue
				}
				// Give each blacklisted name maximum of a second to resolve
				timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 1*time.Second)
				ips, err := inet.NeutralRecursiveResolver.LookupIPAddr(timeoutCtx, name)
				timeoutCancel()
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
	daemon.logger.Info("UpdateBlackList", "", nil,
		"successfully resolved %d blocked IPs from %d domains, the process took %d minutes and used %d parallel routines. The blacklist now contains %d entries in total.",
		countResolvedIPs, len(blacklistedNames), (time.Now().Unix()-beginUnixSec)/60, numRoutines, len(newBlackList))
}

/*
You may call this function only after having called Initialise()!
Start DNS daemon on configured TCP and UDP ports. Block caller until both listeners are told to stop.
If either TCP or UDP port fails to listen, all listeners are closed and an error is returned.
*/
func (daemon *Daemon) StartAndBlock() error {
	// Update ad-block black list in background
	ctx, cancelBlacklistUpdate := context.WithCancel(context.Background())
	defer cancelBlacklistUpdate()
	periodicBlacklistUpdate := &misc.Periodic{
		LogActorName: "dnsd-update-blacklist",
		Interval:     BlacklistInitialDelaySec * time.Second,
		MaxInt:       1,
		Func: func(ctx context.Context, round, _ int) error {
			if round == 0 {
				daemon.logger.Info("UpdateBlacklist", "", nil, "will download blacklists in %d seconds", BlacklistInitialDelaySec)
				select {
				case <-time.After(BlacklistInitialDelaySec * time.Second):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			daemon.UpdateBlackList(DownloadAllBlacklists(BlacklistMaxEntries, daemon.logger))
			return nil
		},
	}
	if err := periodicBlacklistUpdate.Start(ctx); err != nil {
		return err
	}

	// Start server listeners
	numListeners := 0
	errChan := make(chan error, 2)
	if daemon.UDPPort != 0 {
		numListeners++
		go func() {
			err := daemon.udpServer.StartAndBlock()
			errChan <- err
			cancelBlacklistUpdate()
		}()
	}
	if daemon.TCPPort != 0 {
		numListeners++
		go func() {
			err := daemon.tcpServer.StartAndBlock()
			errChan <- err
			cancelBlacklistUpdate()
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
	daemon.tcpServer.Stop()
	daemon.udpServer.Stop()
}

/*
IsInBlacklist returns true only if the input domain name or IP address is black listed. If the domain name represents
a sub-domain name, then the function strips the sub-domain portion in order to check it against black list.
*/
func (daemon *Daemon) IsInBlacklist(nameOrIP string) bool {
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
	daemon.blackListMutex.RLock()
	defer daemon.blackListMutex.RUnlock()
	for _, candidate := range blackListCandidates {
		if _, blacklisted := daemon.blackList[candidate]; blacklisted {
			return true
		}
	}
	return false
}

// TestServer contains the comprehensive test cases for both TCP and UDP DNS servers.
func TestServer(dnsd *Daemon, t testingstub.T) {
	serverStopped := make(chan struct{}, 1)
	go func() {
		if err := dnsd.StartAndBlock(); err != nil {
			t.Errorf("unexpected return value from daemon start: %+v", err)
		}
		serverStopped <- struct{}{}
	}()
	if !misc.ProbePort(5*time.Second, dnsd.Address, dnsd.TCPPort) {
		t.Fatal("did not start within two seconds")
	}

	// Send a simple malformed packet and make sure the daemon will not crash
	tcpClient, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", dnsd.TCPPort))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tcpClient.Write([]byte{0}); err != nil {
		t.Fatal(err)
	}
	if err := tcpClient.Close(); err != nil {
		t.Fatal(err)
	}
	udpClient, err := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", dnsd.UDPPort))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := udpClient.Write([]byte{0}); err != nil {
		t.Fatal(err)
	}
	if err := udpClient.Close(); err != nil {
		t.Fatal(err)
	}

	// Use go DNS client to verify that the server returns satisfactory response
	tcpResolver := &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (conn net.Conn, e error) {
			return net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", dnsd.TCPPort))
		},
	}
	testResolveNameAndBlackList(t, dnsd, tcpResolver)
	udpResolver := &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (conn net.Conn, e error) {
			return net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", dnsd.UDPPort))
		},
	}
	testResolveNameAndBlackList(t, dnsd, udpResolver)

	dnsd.Stop()
	<-serverStopped
	// Repeatedly stopping the daemon should have no negative consequence
	dnsd.Stop()
	dnsd.Stop()
}

/*
testResolveNameAndBlackList is a common test case that tests name resolution of popular domain names as well as black
list domain names.
*/
func testResolveNameAndBlackList(t testingstub.T, daemon *Daemon, resolver *net.Resolver) {
	if platform.HostIsWindows() {
		/*
			As of 2019-08, net.Resolver does not use custom dialer on Windows due to:
			- https://github.com/golang/go/issues/33621 (net: Resolver does not appear to use its dialer function on Windows)
			- https://github.com/golang/go/issues/29621 (net: DNS default resolver vs custom resolver behaviors)
			- https://github.com/golang/go/issues/33086 (net.Resolver Ignores Custom Dialer)
			- https://github.com/golang/go/issues/33097 (proposal: net: Enable built-in DNS stub resolver on Windows)
		*/
		t.Log("due to outstanding issues in Go, DNS server resolution routines cannot be tested on on Windows.")
		return
	}

	// Track and verify the last resolved name
	var lastResolvedName string
	daemon.processQueryTestCaseFunc = func(queryInput string) {
		lastResolvedName = queryInput
	}

	// Resolve A and TXT records from popular domains
	for _, domain := range []string{"bing.com", "wikipedia.org"} {
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
	daemon.blackList["google.com"] = struct{}{}
	if result, err := resolver.LookupIP(context.Background(), "ip4", "GiThUb.CoM"); err != nil || len(result) != 1 || result[0].String() != "0.0.0.0" {
		t.Fatal("failed to get a black-listed response", err, result)
	}
	if lastResolvedName != "GiThUb.CoM" {
		t.Fatal("attempted to resolve black-listed github.com, but daemon saw:", lastResolvedName)
	}
	if result, err := resolver.LookupIP(context.Background(), "ip6", "gooGLE.cOm"); err != nil || len(result) != 1 || result[0].String() != "::1" {
		t.Fatal("failed to get a black-listed response", err, result)
	}
	if lastResolvedName != "gooGLE.cOm" {
		t.Fatal("attempted to resolve black-listed google.com, but daemon saw:", lastResolvedName)
	}

	// Make a TXT query that carries toolbox command prefix but is in fact not
	if result, err := resolver.LookupTXT(context.Background(), "_.apple.com"); err != nil || len(result) == 0 || len(result[0]) == 0 {
		// _.apple.com.            3599    IN      TXT     "v=spf1 redirect=_spf.apple.com"
		t.Fatal(result, err)
	}
	if lastResolvedName != "_.apple.com" {
		t.Fatal("daemon saw the wrong domain name:", lastResolvedName)
	}

	// Make a TXT query that carries toolbox command prefix and an invalid PIN
	appCmdQueryWithBadPassword := "_badpass142s0date.example.com"
	if result, err := resolver.LookupTXT(context.Background(), appCmdQueryWithBadPassword); err == nil || result != nil {
		t.Fatal(result, err)
	}
	if !strings.HasPrefix(lastResolvedName, appCmdQueryWithBadPassword) {
		t.Fatal("daemon saw the wrong domain name:", lastResolvedName)
	}

	// Prefix _ indicates it is a toolbox command, DTMF sequence 142 becomes a full-stop, 0 becomes a space.
	appCmdQueryWithGoodPassword := "_verysecret142s0date.example.com"
	thisYear := strconv.Itoa(time.Now().Year())
	// Make a TXT query that carries toolbox command prefix and a valid command
	result, err := resolver.LookupTXT(context.Background(), appCmdQueryWithGoodPassword)
	if err != nil || len(result) == 0 || !strings.Contains(result[0], thisYear) {
		t.Fatal(result, err)
	}
	if !strings.HasPrefix(lastResolvedName, appCmdQueryWithGoodPassword) {
		t.Fatal("daemon saw the wrong domain name:", lastResolvedName)
	}
	// Rapidly making the same request before TTL period elapses should be met the same command response
	for i := 0; i < 3; i++ {
		if repeatResult, err := resolver.LookupTXT(context.Background(), appCmdQueryWithGoodPassword); err != nil || !reflect.DeepEqual(repeatResult, result) {
			t.Fatal(repeatResult, result, err)
		}
	}
	// Wait for TTL to expire and repeat the same request, it should receive a new response.
	time.Sleep((TextCommandReplyTTL + 1) * time.Second)
	if repeatResult, err := resolver.LookupTXT(context.Background(), appCmdQueryWithGoodPassword); err != nil || reflect.DeepEqual(repeatResult, result) || !strings.Contains(result[0], thisYear) {
		t.Fatal(repeatResult, result, err)
	}
	if !strings.HasPrefix(lastResolvedName, appCmdQueryWithGoodPassword) {
		t.Fatal("daemon saw the wrong domain name:", lastResolvedName)
	}
}
