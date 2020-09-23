package smtpd

import (
	"reflect"
	"strings"
	"testing"
)

func TestGetMailAddressComponents(t *testing.T) {
	name, domain := GetMailAddressComponents("")
	if name != "" || domain != "" {
		t.Fatalf("%s | %s", name, domain)
	}

	name, domain = GetMailAddressComponents("user@example.com")
	if name != "user" || domain != "example.com" {
		t.Fatalf("%s | %s", name, domain)
	}

	name, domain = GetMailAddressComponents("user@")
	if name != "user" || domain != "" {
		t.Fatalf("%s | %s", name, domain)
	}

	name, domain = GetMailAddressComponents("@example.com")
	if name != "" || domain != "example.com" {
		t.Fatalf("%s | %s", name, domain)
	}
}

func TestIsDmarcPolicyEnforcing(t *testing.T) {
	if IsDmarcPolicyEnforcing("") {
		t.Fatal("empty")
	}
	if !IsDmarcPolicyEnforcing("microsoft.com") {
		t.Fatal("microsoft")
	}
	if !IsDmarcPolicyEnforcing("google.com") {
		t.Fatal("google")
	}
	if !IsDmarcPolicyEnforcing("ifttt.com") {
		t.Fatal("ifttt")
	}
	// Test missing DMARC
	if IsDmarcPolicyEnforcing("distrowatch.com") {
		t.Fatal("distrowatch")
	}
	// Test discovery of organisational domain DMARC
	if !IsDmarcPolicyEnforcing("emails.ifttt.com") {
		t.Fatal("emails.ifttt")
	}
	if !IsDmarcPolicyEnforcing("does-not-exist.google.com") {
		t.Fatal("google")
	}
	// Test the artificial limit of how deep the improvised recursive "organisation domain" mechanism goes
	if IsDmarcPolicyEnforcing("1.2.3.4.5.6.7.8.9.microsoft.com") {
		t.Fatal("microsoft recursive over-limit")
	}
}

func TestGetFromAddressWithDmarcWorkaround(t *testing.T) {
	if addr := GetFromAddressWithDmarcWorkaround("", 123); addr != "" {
		t.Fatal(addr)
	}
	// Missing username
	if addr := GetFromAddressWithDmarcWorkaround("microsoft.com", 123); addr != "microsoft.com" {
		t.Fatal(addr)
	}
	// Missing @domain
	if addr := GetFromAddressWithDmarcWorkaround("test-user", 123); addr != "test-user" {
		t.Fatal(addr)
	}
	// DMARC-enforcing domains
	if addr := GetFromAddressWithDmarcWorkaround("user@microsoft.com", 123); addr != "user@microsoft-laitos-nodmarc-123.com" {
		t.Fatal(addr)
	}
	if addr := GetFromAddressWithDmarcWorkaround("user@emails.ifttt.com", 123); addr != "user@emails.ifttt-laitos-nodmarc-123.com" {
		t.Fatal(addr)
	}
	if addr := GetFromAddressWithDmarcWorkaround("user@123.emails.ifttt.com", 123); addr != "user@123.emails.ifttt-laitos-nodmarc-123.com" {
		t.Fatal(addr)
	}
	// Not enforcing DMARC or no DMARC information is available
	if addr := GetFromAddressWithDmarcWorkaround("user@www.b737.org.uk", 123); addr != "user@www.b737.org.uk" {
		t.Fatal(addr)
	}
	if addr := GetFromAddressWithDmarcWorkaround("user@arstechnica.com", 123); addr != "user@arstechnica.com" {
		t.Fatal(addr)
	}
	if addr := GetFromAddressWithDmarcWorkaround("user@distrowatch.com", 123); addr != "user@distrowatch.com" {
		t.Fatal(addr)
	}
}

func TestWithHeaderFromAddr(t *testing.T) {
	if b := WithHeaderFromAddr(nil, "a"); !reflect.DeepEqual(b, []byte{}) {
		t.Fatalf("%+v", b)
	}
	example := `Date: Sun, 24 May 2020 08:48:54 +0000 (UTC)
From: Webhooks via IFTTT <action@ifttt.com>
Reply-To: Do not reply <no-reply@ifttt.com>
Message-ID: <5eca34f1ec12d_2e8a6b3143706f@ip-172-31-1-54.ec2.internal.mail>`
	actual := string(WithHeaderFromAddr([]byte(example), "new@sender.com"))
	if !strings.Contains(actual, "Date: Sun, 24 May 2020 08:48:54 +0000 (UTC)\nFrom: new@sender.com\r\nReply-To: Do not reply <no-reply@ifttt.com>") {
		t.Fatal(actual)
	}
}
