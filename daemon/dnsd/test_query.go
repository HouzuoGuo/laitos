package dnsd

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/stretchr/testify/require"
)

// testQuery validates DNS query responses using go's built in resolver client.
func testQuery(t testingstub.T, daemon *Daemon, resolver *net.Resolver) {
	testAutomaticAuthoritativeResponses(t, daemon, resolver)
	testCustomRecordResolution(t, daemon, resolver)
	testForwardOnlyResolution(t, daemon, resolver)
	testBlacklistedAddresses(t, daemon, resolver)
	testCommandRunner(t, daemon, resolver)
}

// testAutomaticAuthoritativeResponses validates the automatically constructed
// authoritative responses for the DNS daemon's own domain names.
func testAutomaticAuthoritativeResponses(t testingstub.T, daemon *Daemon, resolver *net.Resolver) {
	// Go's name resolution library unfortunately doesn't work for SOA.

	// Resolve A records.
	for _, name := range []string{"example.net", "random.example.com", "random.example.net"} {
		ip, err := resolver.LookupIP(context.Background(), "ip", name)
		if err != nil || len(ip) != 1 {
			t.Fatalf("failed to resolve %q: %v, %v", name, err, ip)
		}
		if !ip[0].Equal(inet.GetPublicIP()) {
			t.Fatalf("got %v, want %v", ip[0], inet.GetPublicIP())
		}
	}

	// Resolve NS records.
	ns, err := resolver.LookupNS(context.Background(), "example.com")
	if len(ns) != 3 || err != nil {
		t.Fatalf("unexpected number of ns: %v, %v", ns, err)
	}
	for i := 1; i < 3; i++ {
		got := ns[i-1].Host
		want := fmt.Sprintf("ns%d.example.com.", i)
		if got != want {
			t.Fatalf("got ns %q, want %q", got, want)
		}
	}

	// Resolve address of NS.
	for i := 1; i < 3; i++ {
		ip, err := resolver.LookupIP(context.Background(), "ip", fmt.Sprintf("ns%d.example.com", i))
		if err != nil {
			t.Fatalf("failed to resolve ns: %v", err)
		}
		if len(ip) != 1 {
			t.Fatalf("did not resolve ns%d", i)
		}
		if !ip[0].Equal(inet.GetPublicIP()) {
			t.Fatalf("got %v, want %v", ip[0], inet.GetPublicIP())
		}
	}

	// Resolve MX.
	mx, err := resolver.LookupMX(context.Background(), "example.com")
	if len(mx) != 1 || err != nil {
		t.Fatalf("unexpected number of mx: %v, %v", mx, err)
	}
	wantMX := &net.MX{Host: "mx.example.com.", Pref: 10}
	if !reflect.DeepEqual(mx[0], wantMX) {
		t.Fatalf("got mx %+v, want mx %+v", mx[0], wantMX)
	}

	// Resolve SPF (TXT).
	txt, err := resolver.LookupTXT(context.Background(), "example.com")
	if len(txt) != 1 || err != nil {
		t.Fatalf("unexpected number of txt: %v", txt, err)
	}
	if txt[0] != `v=spf1 mx a mx:example.com mx:example.net ?all` {
		t.Fatalf("unexpected txt %q", txt[0])
	}
}

// testForwardOnlyResolution validates the DNS daemon as a forward-only stub
// resolver.
func testForwardOnlyResolution(t testingstub.T, daemon *Daemon, resolver *net.Resolver) {
	// Track and verify the last resolved name.
	var lastResolvedName string
	daemon.processQueryTestCaseFunc = func(queryInput string) {
		lastResolvedName = queryInput
	}

	// Resolve A and TXT records from popular domains
	for _, domain := range []string{"biNg.cOM.", "wikipedIA.oRg."} {
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
}

// testBlacklistedAddresses validates the responses of blacklisted names.
func testBlacklistedAddresses(t testingstub.T, daemon *Daemon, resolver *net.Resolver) {
	// Track and verify the last resolved name.
	var lastResolvedName string
	daemon.processQueryTestCaseFunc = func(queryInput string) {
		lastResolvedName = queryInput
	}
	// Blacklist github and see if query gets a black hole response
	oldBlacklist := daemon.blackList
	defer func() {
		daemon.blackList = oldBlacklist
	}()
	daemon.blackList["github.com"] = struct{}{}
	daemon.blackList["google.com"] = struct{}{}
	if result, err := resolver.LookupIP(context.Background(), "ip4", "some.GiThUb.CoM"); err != nil || len(result) != 1 || result[0].String() != "0.0.0.0" {
		t.Fatal("failed to get a black-listed response", err, result)
	}
	if lastResolvedName != "some.GiThUb.CoM." {
		t.Fatal("daemon did not process the query name:", lastResolvedName)
	}
	if result, err := resolver.LookupIP(context.Background(), "ip6", "buzz.gooGLE.cOm"); err != nil || len(result) != 1 || result[0].String() != "::1" {
		t.Fatal("failed to get a black-listed response", err, result)
	}
	if lastResolvedName != "buzz.gooGLE.cOm." {
		t.Fatal("daemon did not process the query name:", lastResolvedName)
	}
}

// testCommandRunner validates app command query responses.
func testCommandRunner(t testingstub.T, daemon *Daemon, resolver *net.Resolver) {
	// Track and verify the last resolved name.
	var lastResolvedName string
	daemon.processQueryTestCaseFunc = func(queryInput string) {
		lastResolvedName = queryInput
	}
	// Make a TXT query that carries toolbox command prefix but is in fact not
	if result, err := resolver.LookupTXT(context.Background(), "_.apple.com"); err != nil || len(result) == 0 || len(result[0]) == 0 {
		// _.apple.com.            3599    IN      TXT     "v=spf1 redirect=_spf.apple.com"
		t.Fatal(result, err)
	}
	if lastResolvedName != "_.apple.com." {
		t.Fatal("daemon saw the wrong domain name:", lastResolvedName)
	}

	// Make a TXT query that carries toolbox command prefix and an invalid PIN
	appCmdQueryWithBadPassword := "_badpass142s0date.example.com"
	if result, err := resolver.LookupTXT(context.Background(), appCmdQueryWithBadPassword); err != nil || len(result) != 1 || result[0] != toolbox.ErrPINAndShortcutNotFound.Error() {
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
	time.Sleep((CommonResponseTTL + 1) * time.Second)
	if repeatResult, err := resolver.LookupTXT(context.Background(), appCmdQueryWithGoodPassword); err != nil || reflect.DeepEqual(repeatResult, result) || !strings.Contains(result[0], thisYear) {
		t.Fatal(repeatResult, result, err)
	}
	if !strings.HasPrefix(lastResolvedName, appCmdQueryWithGoodPassword) {
		t.Fatal("daemon saw the wrong domain name:", lastResolvedName)
	}
}

// testCustomRecordResolution validates query results of custom DNS names.
func testCustomRecordResolution(t testingstub.T, daemon *Daemon, resolver *net.Resolver) {
	netTXT, err := resolver.LookupTXT(context.Background(), "example.net")
	if err != nil || !reflect.DeepEqual(netTXT, []string{`v=spf1 mx a mx:hz.gl mx:howard.gg mx:houzuo.net ?all`, "apple-domain-verification=Abcdefg1234567"}) {
		t.Fatalf("failed to resolve txt: %v, %d, %v", err, len(netTXT), netTXT)
	}

	netMX, err := resolver.LookupMX(context.Background(), "example.net")
	if err != nil || len(netMX) != 2 {
		t.Fatalf("failed to resolve mx: %v, %v", err, netMX)
	}
	if !reflect.DeepEqual(netMX[0], &net.MX{Host: "mx1.example.net.", Pref: 10}) {
		t.Fatalf("wrong mx0: %v", *netMX[0])
	}
	if !reflect.DeepEqual(*netMX[1], net.MX{Host: "mx2.example.net.", Pref: 20}) {
		t.Fatalf("wrong mx1: %v", *netMX[1])
	}

	otherNS, err := resolver.LookupNS(context.Background(), "ns-other.example.net")
	if err != nil || len(otherNS) != 1 {
		t.Fatalf("failed to resolve ns: %v, %v", err, otherNS)
	}
	if !reflect.DeepEqual(otherNS[0], &net.NS{Host: "ns.other.example.com."}) {
		t.Fatalf("wrong ns0: %v", *otherNS[0])
	}

	// Go DNS resolver doesn't work properly for CNAME.
	// otherCNAME, err := resolver.LookupCNAME(context.Background(), "ns-other.example.net")

	comV4Addr, err := resolver.LookupIP(context.Background(), "ip4", "example.com")
	require.NoError(t, err)
	require.ElementsMatch(t, []net.IP{net.IP{5, 0, 0, 1}, net.IP{5, 0, 0, 2}}, comV4Addr)

	comV6Addr, err := resolver.LookupIP(context.Background(), "ip6", "example.com")
	require.NoError(t, err)
	require.ElementsMatch(t, []net.IP{net.ParseIP("1::5"), net.ParseIP("1::6")}, comV6Addr)
}
