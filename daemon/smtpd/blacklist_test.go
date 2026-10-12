package smtpd

import (
	"net"
	"testing"
)

func TestGetBlacklistLookupName(t *testing.T) {
	var tests = []struct {
		ip         string
		lookupName string // empty means the suspect IP is expected to be rejected with an error
	}{
		{"1.2.3.4", "4.3.2.1.example.com"},
		{"252.253.254.255", "255.254.253.252.example.com"},
		// An IPv4-mapped IPv6 address is looked up as its IPv4 counterpart.
		{"::ffff:1.2.3.4", "4.3.2.1.example.com"},
		// An IPv6 address is looked up as the 32 hex nibbles of the address in reverse order (RFC 5782 section 2.4).
		{"2001:db8::1", "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.example.com"},
		{"2001:db8:7ca6:22::45", "5.4.0.0.0.0.0.0.0.0.0.0.0.0.0.0.2.2.0.0.6.a.c.7.8.b.d.0.1.0.0.2.example.com"},
		{"::1", "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.example.com"},
		{"not-a-valid-ip4-addr", ""},
		{"not-a-valid-ip6-addr", ""},
		{"2001:db8::1::2", ""},
	}
	for _, test := range tests {
		lookupName, err := GetBlacklistLookupName(test.ip, "example.com")
		if test.lookupName == "" {
			if err == nil {
				t.Fatalf("%s should have been rejected with an error, but it was looked up as %s", test.ip, lookupName)
			}
		} else if err != nil || lookupName != test.lookupName {
			t.Fatalf("%s should have been looked up as %s, but it was looked up as %s (%v)", test.ip, test.lookupName, lookupName, err)
		}
	}
}

func TestIsIPBlacklistIndication(t *testing.T) {
	var tests = []struct {
		ip       string
		expected bool
	}{
		{"127.0.0.0", true},
		{"127.0.0.1", true},
		{"127.0.0.8", true},
		{"127.0.1.0", true},
		{"127.1.0.1", false},
		{"127.254.254.254", false},
		{"1.1.1.1", false},
		{"192.168.0.1", false},
	}
	for _, test := range tests {
		if IsIPBlacklistIndication(net.ParseIP(test.ip)) != test.expected {
			t.Fatalf("return value for %s should have been %v", test.ip, test.expected)
		}
	}
}

func TestIsClientBlacklisted(t *testing.T) {
	var tests = []struct {
		ip, blockedBy string
	}{

		{"1.1.2.2", ""},
		{"1.0.0.2", ""},
		{"8.8.8.8", ""},
		{"8.8.9.9", ""},

		{"127.0.0.1", ""},
		{"127.254.254.254", ""},
		{"192.168.0.1", ""},
		{"192.168.254.254", ""},

		{"::1", ""},
		{"2001:4860:4860::8844", ""},

		{"not-a-valid-ip-addr", ""},
	}
	for _, test := range tests {
		if blockedBy := IsSuspectIPBlacklisted(test.ip); blockedBy != test.blockedBy {
			t.Fatalf(`%s should have been blocked by "%v" (can be empty), it is actually blocked by "%v"`, test.ip, test.blockedBy, blockedBy)
		}
	}
	// Is there an IP guaranteed to be blocked for sending spam?!
}
