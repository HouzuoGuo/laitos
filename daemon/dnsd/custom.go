package dnsd

import (
	"fmt"
	"net"
	"strings"
)

// lintDNSName improves the protocol conformance of the input name and returns it.
func lintDNSName(in string) string {
	in = strings.TrimSpace(strings.ToLower(in))
	if len(in) == 0 {
		return in
	}
	if !strings.HasSuffix(in, ".") {
		in += "."
	}
	return in
}

// CustomRecord defines various types of resource records for a complete DNS name (e.g. www.me.example.com).
type CustomRecord struct {
	A    V4AddressRecord `json:"A"`
	AAAA V6AddressRecord `json:"AAAA"`
	TXT  TextRecord      `json:"TXT"`
	MX   []*net.MX       `json:"MX"`
	NS   NSRecord        `json:"NS"`
}

func (dname *CustomRecord) MXExists() bool {
	return len(dname.MX) > 0
}

// Lint checks all records for errors, and modifies them in-place to conform to protocol requirements.
func (dname *CustomRecord) Lint() error {
	if err := dname.A.Lint(); err != nil {
		return err
	}
	if err := dname.AAAA.Lint(); err != nil {
		return err
	}
	if err := dname.TXT.Lint(); err != nil {
		return err
	}
	for i, rec := range dname.MX {
		if rec == nil || strings.TrimSpace(rec.Host) == "" {
			return fmt.Errorf("MX record at index %v must have a host name", i)
		}
		rec.Host = lintDNSName(rec.Host)
	}
	if err := dname.NS.Lint(); err != nil {
		return err
	}
	return nil
}

// AddressRecord represents an version-agnostic DNS address record that has either IP addresses or CNAME.
type AddressRecord struct {
	Addresses     []string `json:"Addresses"`
	CanonicalName string   `json:"CanonicalName"`
	ipAddresses   []net.IP
}

func (rec *AddressRecord) Exists() bool {
	return len(rec.Addresses) > 0 || rec.CanonicalName != ""
}

// Lint checks the record for errors, and modifies it in-place to conform to protocol requirements.
func (rec *AddressRecord) Lint(ipNetwork string) error {
	rec.CanonicalName = lintDNSName(rec.CanonicalName)
	if len(rec.Addresses) > 0 && rec.CanonicalName != "" {
		return fmt.Errorf("the record must have either addresses (%v) or canonical name (%q) but not both", rec.Addresses, rec.CanonicalName)
	}
	rec.ipAddresses = nil
	for _, addr := range rec.Addresses {
		ipAddr, err := net.ResolveIPAddr(ipNetwork, strings.TrimSpace(addr))
		if err != nil {
			return fmt.Errorf("failed to parse IP address %q: %w", addr, err)
		}
		rec.ipAddresses = append(rec.ipAddresses, ipAddr.IP)
	}
	return nil
}

// V4AddressRecord is an IPv4 or CNAME address record.
type V4AddressRecord struct {
	AddressRecord
}

// Lint checks the record for errors, and modifies it in-place to conform to protocol requirements.
func (a *V4AddressRecord) Lint() error {
	return a.AddressRecord.Lint("ip4")
}

// V46ddressRecord is an IPv6 or CNAME address record.
type V6AddressRecord struct {
	AddressRecord
}

// Lint checks the record for errors, and modifies it in-place to conform to protocol requirements.
func (aaaa *V6AddressRecord) Lint() error {
	return aaaa.AddressRecord.Lint("ip6")
}

// TextRecord is a TXT record.
type TextRecord struct {
	Entries []string `json:"Entries"`
}

func (rec *TextRecord) Exists() bool {
	return len(rec.Entries) > 0
}

// Lint checks the record for errors, and modifies it in-place to conform to protocol requirements.
func (txt *TextRecord) Lint() error {
	for i, entry := range txt.Entries {
		entry = strings.TrimSpace(entry)
		if len(entry) == 0 {
			return fmt.Errorf("text record entry at index %d must have content", i)
		} else if len(entry) > 254 {
			return fmt.Errorf("text record %q must be shorter than 255 characters", entry)
		}
		txt.Entries[i] = entry
	}
	return nil
}

type NSRecord struct {
	Names []string `json:"Names"`
}

// Lint checks the record for errors, and modifies it in-place to conform to protocol requirements.
func (ns NSRecord) Lint() error {
	for i, name := range ns.Names {
		name = lintDNSName(name)
		ns.Names[i] = name
		if name == "" {
			return fmt.Errorf("name server entry at index %d must have a dns name", i)
		}
	}
	return nil
}

func (rec *NSRecord) Exists() bool {
	return len(rec.Names) > 0
}
