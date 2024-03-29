package smtpd

import (
	"net"
	"testing"
)

func TestGetBlacklistLookupName(t *testing.T) {
	if toLookup, err := GetBlacklistLookupName("1.2.3.4", "example.com"); err != nil || toLookup != "4.3.2.1.example.com" {
		t.Fatal(toLookup, err)
	}
	if toLookup, err := GetBlacklistLookupName("252.253.254.255", "example.com"); err != nil || toLookup != "255.254.253.252.example.com" {
		t.Fatal(toLookup, err)
	}
	if toLookup, err := GetBlacklistLookupName("not-a-valid-ip4-addr", "example.com"); err == nil {
		t.Fatal(toLookup, err)
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

		{"not-a-valid-ipv4-addr", ""},
	}
	for _, test := range tests {
		if blockedBy := IsSuspectIPBlacklisted(test.ip); blockedBy != test.blockedBy {
			t.Fatalf(`%s should have been blocked by "%v" (can be empty), it is actually blocked by "%v"`, test.ip, test.blockedBy, blockedBy)
		}
	}
	// Is there an IP guaranteed to be blocked for sending spam?!
}
