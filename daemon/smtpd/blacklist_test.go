package smtpd

import "testing"

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

func TestIsClientBlacklisted(t *testing.T) {
	if IsClientIPBlacklisted("not-a-valid-ipv4-addr") {
		t.Fatal("should not have blacklisted")
	}
	if IsClientIPBlacklisted("1.1.1.1") {
		t.Fatal("should not have blacklisted")
	}
	// Is there an IP guaranteed to be blocked for sending spam?!
}
