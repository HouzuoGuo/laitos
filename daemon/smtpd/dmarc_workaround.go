package smtpd

import (
	"bytes"
	"fmt"
	"net"
	"strings"
)

/*
DmarcWorkaroundDomainPrefix is the prefix string prepended to mail's From address when the address domain name has strong DMARC
verification requirement. By adding the prefix to the forwarded mail, the recipient will not perform a DMARC verification,
enhancing the chance of mail delivery at the expense of sacrificing message authenticity.
*/
const DmarcWorkaroundDomainPrefix = "laitos-nodmarc"

/*
GetMailAddressComponents returns the mail address (e.g. "name@example.com") broken down into its name and domain name components.
If a component is not present in the input address, the function will return an empty string for that component.
*/
func GetMailAddressComponents(addr string) (name, domain string) {
	indexAt := strings.IndexRune(addr, '@')
	if indexAt == -1 {
		return
	}
	name = strings.TrimSpace(addr[:indexAt])
	if indexAt < len(addr)-1 {
		domain = strings.TrimSpace(addr[indexAt+1:])
	}
	return
}

/*
IsDmarcPolicyEnforcing returns true only if the domain name demands quarantine or rejection from failed DMARC verification.
If DMARC policy cannot be determined, the function will return false.
*/
func IsDmarcPolicyEnforcing(domain string) bool {
	records, err := net.LookupTXT("_dmarc." + domain)
	if err == nil {
		/*
			The TXT query was resolved successfully.
			LookupTXT makes it quite difficult to determine whether an error originates from IO failure or no-such-host.
		*/
		var seenDmarc bool
		for _, record := range records {
			record = strings.ToLower(record)
			if !strings.Contains(record, "v=dmarc") {
				continue
			}
			seenDmarc = true
			if strings.Contains(record, "p=quarantine") || strings.Contains(record, "p=reject") {
				return true
			}
		}
		// The domain or sub-domain does not demand quarantine or rejection
		if seenDmarc {
			return false
		}
	}
	/*
		If the domain name is a sub-domain and does not have a DMARC policy, then look for DMARC policy from its "organisational domain".
		For example, the organisation domain of "sub1.sub2.example.co.uk" would be "example.co.uk".
		As I do not have a comprehensive list of all public suffixes, I will just make this check recursive, making at most 7 recursive
		attempts (1.2.3.4.5.6.7.example.com) at reaching the organisational domain.
	*/
	if c := strings.Count(domain, "."); c > 1 && c < 8 {
		indexDot := strings.IndexRune(domain, '.')
		if indexDot < len(domain)-4 {
			// the shortest possible domain name is 3+1 characters long (e.g. t.co)
			return IsDmarcPolicyEnforcing(domain[indexDot+1:])
		}
	}
	return false
}

/*
GetFromAddressWithDmarcWorkaround returns an altered mail From address that contains a workaround prefix string in its domain name
portion, only if the original domain name enforces DMARC verification.
The modified domain name prevents the recipient of this mail from performing DMARC verification, which means, laitos has a better
chance at delivering this mail to the recipient, at the expense of sacrificing message authenticity.
If the domain name does not enforce DMARC verification, the function will return the verbatim address.
*/
func GetFromAddressWithDmarcWorkaround(fromAddr string, randNum int) string {
	name, domain := GetMailAddressComponents(fromAddr)
	if name == "" || domain == "" {
		return fromAddr
	}
	if !IsDmarcPolicyEnforcing(domain) {
		return fromAddr
	}
	return fmt.Sprintf("%s@%s-%d-%s", name, DmarcWorkaroundDomainPrefix, randNum, domain)
}

// WithHeaderFromAddr changes the "From:" header value into the input string and returns the new message.
func WithHeaderFromAddr(mail []byte, newFromAddr string) []byte {
	if len(mail) == 0 {
		return []byte{}
	}
	// Look no further than 16KB for the "From" header
	searchLimit := len(mail)
	if searchLimit > 16*1024 {
		searchLimit = 16 * 1024
	}
	/*
		The Internet Message Format RFC does not say much about header's case sensitivity:
		https://tools.ietf.org/html/rfc5322#section-2.2
	*/
	var fromHeaderIndex int
	fromHeaderIndex = bytes.Index(mail[:searchLimit], []byte("From:"))
	if fromHeaderIndex == -1 {
		fromHeaderIndex = bytes.Index(mail[:searchLimit], []byte("from:"))
		if fromHeaderIndex == -1 {
			fromHeaderIndex = bytes.Index(mail[:searchLimit], []byte("FROM:"))
		}
	}
	if fromHeaderIndex == -1 {
		// This is a malformed mail message without a From header
		return mail
	}
	// Look for end (LF) of the header line
	lf := bytes.IndexByte(mail[fromHeaderIndex:searchLimit], 0x0a)
	if lf == -1 {
		// This is a malformed mail message with an exceedingly long From header
		return mail
	}
	lf += fromHeaderIndex
	/*
		Change everything in between "From:" and LF
		Remember to put a CR in front of the LF.
	*/
	alteredMail := append(mail[:fromHeaderIndex], append([]byte(fmt.Sprintf("From: %s\r\n", newFromAddr)), mail[lf+1:]...)...)
	return alteredMail
}
