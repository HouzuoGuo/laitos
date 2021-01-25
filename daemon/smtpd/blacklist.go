package smtpd

import (
	"context"
	"fmt"
	"net"
	"time"
)

var (
	// SpamBlacklistLookupServers is a list of domain names that provide email spam reporting and blacklist look-up services.
	// Each of the domain name offers a DNS-based blacklist look-up service. By appending the reversed IPv4 address to any of
	// the domain names (e.g. resolve 4.3.2.1.domain.net to check blacklist status of 1.2.3.4), the success of DNS resolution
	// will indictate that the IP address has been blacklisted for spamming.
	SpamBlacklistLookupServers = []string{"dnsbl.sorbs.net", "bl.spamcop.net"}
)

// GetBlacklistLookupName returns a DNS name constructed from a combination of the suspect IP and blacklist
// lookup domain name. For example, in order to look-up a suspect IP 1.2.3.4 using blacklist look-up domain
// bl.spamcop.net, the function will return "4.3.2.1.bl.spamcop.net".
// The caller should then attempt to resolve the A record of the returned name. If the resolution is successful,
// then the suspect IP has been blacklisted by the look-up domain.
func GetBlacklistLookupName(suspectIP, blLookupDomain string) (string, error) {
	suspectIPv4 := net.ParseIP(suspectIP).To4()
	if suspectIPv4 == nil || len(suspectIPv4) < 4 {
		return "", fmt.Errorf("GetBlacklistLookupName: suspect IP %s does not appear to be a valid IPv4 address", suspectIP)
	}
	return fmt.Sprintf("%d.%d.%d.%d.%s", suspectIPv4[3], suspectIPv4[2], suspectIPv4[1], suspectIPv4[0], blLookupDomain), nil
}

// IsClientIPBlacklisted looks up the suspect IP from all sources of spam blacklists. If the suspect IP is blacklisted by any
// of the spam blacklists, then the function will return true. If the suspect IP is not blacklisted or due to network error
// the blacklist status cannot be determined, then the function will return false.
func IsClientIPBlacklisted(suspectIP string) bool {
	blacklisted := make(chan bool, 2)
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer timeoutCancel()
	for _, lookupDomain := range SpamBlacklistLookupServers {
		go func(lookupDomain string) {
			lookupName, err := GetBlacklistLookupName(suspectIP, lookupDomain)
			if err != nil {
				// Cannot possibly blacklist an invalid client IP
				blacklisted <- false
				return
			}
			fmt.Println("looking up", lookupName)
			_, err = net.DefaultResolver.LookupIPAddr(timeoutCtx, lookupName)
			// Successful DNS resolution means the client IP is in blacklist
			blacklisted <- err == nil
		}(lookupDomain)
	}
	select {
	case <-timeoutCtx.Done():
		return false
	case ret := <-blacklisted:
		return ret
	}
}
