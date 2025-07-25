package dnsd

import (
	"context"
	"fmt"
	"net"
	"sort"
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
	ForwarderTimeoutSec         = 1 * 2     // ForwarderTimeoutSec is the IO timeout for a round trip interaction with forwarders
	ClientTimeoutSec            = 30 * 2    // AnswerTimeoutSec is the IO timeout for a round trip interaction with DNS clients
	MaxPacketSize               = 9038      // Maximum acceptable UDP packet size
	BlacklistUpdateIntervalSec  = 12 * 3600 // Update ad-server blacklist at this interval
	BlacklistInitialDelaySec    = 120       // BlacklistInitialDelaySec is the number of seconds to wait for downloading blacklists for the first time.
	MinNameQuerySize            = 14        // If a query packet is shorter than this length, it cannot possibly be a name query.
	PublicIPRefreshIntervalSec  = 900       // PublicIPRefreshIntervalSec is how often the program places its latest public IP address into array of IPs that may query the server.
	BlackListDownloadTimeoutSec = 30        // BlackListDownloadTimeoutSec is the timeout to use when downloading blacklist hosts files.
	BlacklistMaxEntries         = 100000    // BlackListMaxEntries is the maximum number of entries to be accepted into black list after retireving them from public sources.
	// CommonResponseTTL is the TTL of outgoing authoritative response records.
	CommonResponseTTL = 60
	/*
		ToolboxCommandPrefix is a short string that indicates a TXT query is most likely toolbox command. Keep it short,
		as DNS query input has to be pretty short.
	*/
	ToolboxCommandPrefix = '_'

	// ProxyPrefix is the name prefix DNS clients need to put in front of their
	// address queries to send the query to the TCP-over-DNS proxy.
	ProxyPrefix = 't'
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
	Address string `json:"Address"` // Network address for both TCP and UDP to listen to, e.g. 0.0.0.0 for all network interfaces.
	// AllowQueryFromCidrs are the network address blocks (both IPv4 and IPv6)
	// from which clients may send recursive queries.
	// Queries that are directed at DNS server's own domain names
	// (MyDomainNames) are not restricted by this list.
	AllowQueryFromCidrs []string `json:"AllowQueryFromCidrs"`
	// PerIPLimit is the approximate number of UDP packets and TCP connections accepted from each client IP per second.
	// The limit needs to be sufficiently high for TCP-over-DNS queries.
	PerIPLimit int `json:"PerIPLimit"`
	// PerIPQueryLimit is the approximate number of DNS queries (excluding TCP-over-DNS) processed from each client IP per second.
	PerIPQueryLimit int `json:"PerIPQueryLimit"`
	// Forwarders are recursive DNS resolvers for all query types. All resolvers
	// must support both TCP and UDP.
	Forwarders []string `json:"Forwarders"`
	// Processor enables execution of toolbox commands via DNS TXT queries when
	// the queries are directed at the server's own domain name(s).
	Processor *toolbox.CommandProcessor `json:"-"`
	// MyDomainNames lists the domain names belonging to laitos server itself.
	// When laitos DNS server is used as these domains' name servers, the DNS
	// server will automaitcally respond authoritatively to SOA, NS, MX, and A
	// requests for the domains.
	// This is especially important to support TCP-over-DNS usage and DNS app
	// command runner requests.
	// CustomRecords take precedence over these automatically constructed
	// responses. For all other domain names, the DNS server works as a stub
	// forward-only resolver.
	MyDomainNames []string `json:"MyDomainNames"`
	// CustomRecords are the user-defined DNS records for which the DNS server
	// will respond authoritatively.
	CustomRecords map[string]*CustomRecord `json:"CustomRecords"`

	// SafeBrowsing when true will download and update ad/malware blacklists in the background, the DNS daemon will resolve the names to 0.0.0.0.
	// In addition, the HTTP proxy daemon and socks daemon will block connection attempts toward the IPs resolved from the blacklists.
	SafeBrowsing bool `json:"SafeBrowsing"`

	UDPPort int `json:"UDPPort"` // UDP port to listen on
	TCPPort int `json:"TCPPort"` // TCP port to listen on

	tcpServer      *common.TCPServer
	udpServer      *common.UDPServer
	queryRateLimit *lalog.RateLimit

	// TCPProxy is a TCP-over-DNS proxy server.
	TCPProxy *Proxy `json:"TCPProxy"`
	// DNSRelay provides a transport for forwarded queries.
	DNSRelay *DNSRelay `json:"-"`

	// latestCommands caches the result of recently executed toolbox commands.
	latestCommands *LatestCommands
	// responseCache caches the responses of recently made queries.
	responseCache *ResponseCache
	// processQueryTestCaseFunc works along side DNS query processing routine, it offers queried name to test case for inspection.
	processQueryTestCaseFunc func(string)

	// blackList is a map of domain names (in lower case) and IP addresses
	// they resolve into. For DNS queries targeting these domain names, the DNS
	// server will respond to the queries with a blackhole address e.g. 0.0.0.0.
	//
	// The IP addresses are not used by this daemon, instead they are used by
	// other daemons that have built-in blacklist capability for IP addresses,
	// such as the HTTP proxy and sockd.
	blackList      map[string]struct{}
	blackListMutex *sync.RWMutex

	context                context.Context
	cancelFunc             func()
	logger                 *lalog.Logger
	allowQueryFromCidrNets []*net.IPNet
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
		if daemon.TCPProxy == nil {
			// This should be good enough for a small network of 5 users.
			daemon.PerIPLimit = 50
		} else {
			// TCP-over-DNS sends a LOT of queries.
			daemon.PerIPLimit = 300
		}
	}
	if daemon.PerIPQueryLimit < 1 {
		// This should be good enough for a small network of 5 users.
		daemon.PerIPQueryLimit = 50
	}
	if len(daemon.Forwarders) == 0 {
		daemon.Forwarders = make([]string, len(DefaultForwarders))
		copy(daemon.Forwarders, DefaultForwarders)
	}
	daemon.logger = &lalog.Logger{
		ComponentName: "dnsd",
		ComponentID:   []lalog.LoggerIDField{{Key: "TCP", Value: daemon.TCPPort}, {Key: "UDP", Value: daemon.UDPPort}},
	}
	if daemon.Processor == nil || daemon.Processor.IsEmpty() {
		daemon.logger.Info("", nil, "daemon will not be able to execute toolbox commands due to lack of command processor filter configuration")
		daemon.Processor = toolbox.GetEmptyCommandProcessor()
	}
	if len(daemon.MyDomainNames) == 0 {
		daemon.logger.Info("", nil, "daemon will not be able to execute toolbox commands because MyDomainNames is empty")
		daemon.Processor = toolbox.GetEmptyCommandProcessor()
	}
	for i, name := range daemon.MyDomainNames {
		if len(name) < 3 {
			return fmt.Errorf("Initialise: MyDomainNames contains an invalid entry %q", name)
		}
		// Remove the full-stop suffix and give it a full-stop prefix.
		// This satisfies the expectation of Daemon.queryLabels.
		if name[len(name)-1] == '.' {
			name = name[:len(name)-1]
		}
		if name[0] != '.' {
			name = "." + name
		}
		daemon.MyDomainNames[i] = name
	}
	// Sort the domain name from the longest to shortest, this allows the
	// longest domain name to be first matched against an incoming query.
	sort.Slice(daemon.MyDomainNames, func(i, j int) bool {
		return len(daemon.MyDomainNames[i]) > len(daemon.MyDomainNames[j])
	})
	for dnsName, records := range daemon.CustomRecords {
		if lintDNSName(dnsName) == "" {
			return fmt.Errorf("Initialise: CustomRecords must not use an empty DNS name")
		}
		if err := records.Lint(); err != nil {
			return fmt.Errorf("Initialise: custom record error - %w", err)
		}
	}

	if errs := daemon.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("dnsd.Initialise: %+v", errs)
	}
	daemon.Processor.SetLogger(daemon.logger)
	daemon.allowQueryFromCidrNets = make([]*net.IPNet, 0)
	for _, cidr := range daemon.AllowQueryFromCidrs {
		_, cidrNet, err := net.ParseCIDR(cidr)
		if err != nil || cidr == "" {
			return fmt.Errorf("Initialise: failed to parse AllowQueryFromCidrs entry %q", cidr)
		}
		daemon.allowQueryFromCidrNets = append(daemon.allowQueryFromCidrNets, cidrNet)
	}

	daemon.blackListMutex = new(sync.RWMutex)
	daemon.blackList = make(map[string]struct{})

	daemon.latestCommands = NewLatestCommands()
	daemon.responseCache = NewResponseCache(5*time.Second, 200)
	daemon.tcpServer = common.NewTCPServer(daemon.Address, daemon.TCPPort, "dnsd", daemon, daemon.PerIPLimit)
	daemon.udpServer = common.NewUDPServer(daemon.Address, daemon.UDPPort, "dnsd", daemon, daemon.PerIPLimit)
	daemon.queryRateLimit = lalog.NewRateLimit(1, daemon.PerIPQueryLimit, daemon.logger)
	if daemon.TCPProxy != nil && daemon.TCPProxy.RequestOTPSecret != "" {
		daemon.TCPProxy.DNSDaemon = daemon
	}
	return nil
}

// isRecursiveQueryAllowed checks whether the input client IP is allowed to make
// recursive queries to this DNS server.
func (daemon *Daemon) isRecursiveQueryAllowed(clientIP string) bool {
	if clientIP == "" || len(clientIP) > 64 {
		return false
	}
	// Fast track - always allow this host to query itself.
	if strings.HasPrefix(clientIP, "127.") || clientIP == "::1" || clientIP == inet.GetPublicIP().String() {
		return true
	}
	// Another fast track - monitored subjects phoning home are allowed.
	// By convention, the subject reports arriving via IP network will have
	// their client IP address recorded in the report tag attribute.
	if daemon.Processor.Features.MessageProcessor.HasClientTag(clientIP) {
		return true
	}
	// Allow clients from whitelisted CIDR blocks to query.
	parsedClientIP := net.ParseIP(clientIP)
	for _, cidrNet := range daemon.allowQueryFromCidrNets {
		if cidrNet.Contains(parsedClientIP) {
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
	numRoutines := 8
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
					daemon.logger.Info("", nil, "resolving %d of %d black listed domain names",
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
	daemon.logger.Info("", nil, "successfully resolved %d blocked IPs from %d domains, the process took %d minutes and used %d parallel routines. The blacklist now contains %d entries in total.",
		countResolvedIPs, len(blacklistedNames), (time.Now().Unix()-beginUnixSec)/60, numRoutines, len(newBlackList))
}

/*
You may call this function only after having called Initialise()!
Start DNS daemon on configured TCP and UDP ports. Block caller until both listeners are told to stop.
If either TCP or UDP port fails to listen, all listeners are closed and an error is returned.
*/
func (daemon *Daemon) StartAndBlock() error {
	daemon.context, daemon.cancelFunc = context.WithCancel(context.Background())
	// Update ad-block black list in background
	ctx, cancelBlacklistUpdate := context.WithCancel(context.Background())
	defer cancelBlacklistUpdate()
	periodicBlacklistUpdate := &misc.Periodic{
		LogActorName: "dnsd-update-blacklist",
		Interval:     BlacklistUpdateIntervalSec * time.Second,
		MaxInt:       1,
		Func: func(ctx context.Context, round, _ int) error {
			if daemon.SafeBrowsing {
				if round == 0 {
					daemon.logger.Info("", nil, "will download blacklists in %d seconds", BlacklistInitialDelaySec)
					select {
					case <-time.After(BlacklistInitialDelaySec * time.Second):
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				daemon.UpdateBlackList(DownloadAllBlacklists(BlacklistMaxEntries, daemon.logger))

			} else {
				daemon.logger.Info("", nil, "safe browsing is not enabled, will not updated dns & ip blacklists.")
			}
			return nil
		},
	}
	if daemon.DNSRelay == nil {
		// Update the blacklist periodically only when running a regular DNS
		// server.
		if err := periodicBlacklistUpdate.Start(ctx); err != nil {
			return err
		}
	} else {
		if err := daemon.DNSRelay.Initialise(daemon.context); err != nil {
			return err
		}
		go func() {
			if err := daemon.DNSRelay.StartAndBlock(); err != nil {
				daemon.logger.Warning(nil, err, "the DNS relay has stopped, will now stop the DNS server as well.")
				daemon.Stop()
			}
		}()
	}
	if daemon.TCPProxy != nil && daemon.TCPProxy.RequestOTPSecret != "" {
		daemon.TCPProxy.Start(daemon.context)
	}

	// Start the DNS listeners on all ports.
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
	daemon.cancelFunc()
	daemon.tcpServer.Stop()
	daemon.udpServer.Stop()
}

/*
IsInBlacklist returns true only if the input domain name or IP address is black listed. If the domain name represents
a sub-domain name, then the function strips the sub-domain portion in order to check it against black list.
*/
func (daemon *Daemon) IsInBlacklist(nameOrIP string) bool {
	// Treat excessively (impossibly) long input name as if it is black-listed.
	if len(nameOrIP) > 255 || len(nameOrIP) < 4 {
		return true
	}
	// The black list uses lower case letters by convention.
	nameOrIP = strings.ToLower(strings.TrimSpace(nameOrIP))
	// Trim the rightmost dot.
	if nameOrIP[len(nameOrIP)-1] == '.' {
		nameOrIP = nameOrIP[:len(nameOrIP)-1]
	}
	// Discover all candidate names derived from the input name that can be used
	// to find a blacklist match.
	// If "a.com" is blacklisted, then "alpha.a.com" and "beta.alpha.a.com" are
	// also considered blacklisted.
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

// queryLabels helps caller process an input DNS name by dissecting it into
// labels and the domain name as it originally appeared (case sensitive), and
// determine whether a custom record match exists, or whether the query should
// be forwarded to a recursive resolver.
// All labels and names among the return values preserve the case sensitivity
// consistent with the query name.
func (daemon *Daemon) queryLabels(queryName string) (labelsExclDomain []string, queryDomainName string, numDomainLabels int, isRecursive bool, customRecord *CustomRecord) {
	if len(queryName) < 3 {
		return []string{}, "", 0, false, nil
	}
	// Remove the suffix full-stop to aid in matching daemon's own domain names
	// (e.g. ".example.com").
	if queryName[len(queryName)-1] == '.' {
		queryName = queryName[:len(queryName)-1]
	}
	// Add a prefix full-stop to aid in matching daemon's own domain names (e.g.
	// ".example.com").
	if queryName[0] != '.' {
		queryName = "." + queryName
	}
	lowerNameFullStop := strings.ToLower(queryName)
	isRecursive = true
	// Remove all configured domain suffixes from the queried name.
	nameExclDomain := queryName
	for _, suffix := range daemon.MyDomainNames {
		if strings.HasSuffix(lowerNameFullStop, suffix) {
			// The suffix has a trailing full-stop.
			isRecursive = false
			nameExclDomain = queryName[:len(queryName)-len(suffix)]
			queryDomainName = queryName[len(nameExclDomain)+1:]
			numDomainLabels = CountNameLabels(queryDomainName)
			break
		}
	}
	// Match against a custom defined record.
	if record, exists := daemon.CustomRecords[lowerNameFullStop[1:]]; exists && record != nil {
		isRecursive = false
		customRecord = record
	}
	labelsExclDomain = strings.Split(nameExclDomain, ".")[1:]
	return
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
	if !misc.ProbePort(30*time.Second, dnsd.Address, dnsd.TCPPort) {
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
	testQuery(t, dnsd, tcpResolver)
	udpResolver := &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (conn net.Conn, e error) {
			return net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", dnsd.UDPPort))
		},
	}
	testQuery(t, dnsd, udpResolver)

	dnsd.Stop()
	<-serverStopped
	// Repeatedly stopping the daemon should have no negative consequence
	dnsd.Stop()
	dnsd.Stop()
}
