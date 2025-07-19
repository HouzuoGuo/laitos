package dnsd

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/tcpoverdns"
	"github.com/HouzuoGuo/laitos/toolbox"
	"golang.org/x/net/dns/dnsmessage"
)

const (
	// EDNSBufferSize is the maximum DNS buffer size advertised to DNS clients.
	EDNSBufferSize = 1232
)

// BuildTextResponse constructs a TXT record response packet.
func BuildTextResponse(name string, header dnsmessage.Header, question dnsmessage.Question, txt []string) ([]byte, error) {
	// Retain the original transaction ID.
	header.Response = true
	header.Truncated = false
	header.Authoritative = true
	header.RecursionAvailable = header.RecursionDesired
	builder := dnsmessage.NewBuilder(nil, header)
	builder.EnableCompression()
	// Repeat the question back to the client, this is required by DNS protocol.
	if err := builder.StartQuestions(); err != nil {
		return nil, err
	}
	if err := builder.Question(question); err != nil {
		return nil, err
	}
	if err := builder.StartAnswers(); err != nil {
		return nil, err
	}
	dnsName, err := dnsmessage.NewName(name)
	if err != nil {
		return nil, err
	}
	for _, entry := range txt {
		if err := builder.TXTResource(dnsmessage.ResourceHeader{
			Name:  dnsName,
			Class: dnsmessage.ClassINET, TTL: CommonResponseTTL}, dnsmessage.TXTResource{TXT: []string{entry}}); err != nil {
			return nil, err
		}
	}
	return builder.Finish()
}

// BuildBlackHoleAddrResponse constructs an A or AAAA address record response
// packet pointing to localhost, the record TTL is hard coded to 600 seconds.
func BuildBlackHoleAddrResponse(header dnsmessage.Header, question dnsmessage.Question) ([]byte, error) {
	// Retain the original transaction ID.
	header.Response = true
	header.Truncated = false
	header.Authoritative = true
	header.RecursionAvailable = header.RecursionDesired
	builder := dnsmessage.NewBuilder(nil, header)
	builder.EnableCompression()
	// Repeat the question back to the client, this is required by DNS protocol.
	if err := builder.StartQuestions(); err != nil {
		return nil, err
	}
	if err := builder.Question(question); err != nil {
		return nil, err
	}
	if err := builder.StartAnswers(); err != nil {
		return nil, err
	}
	dnsName, err := dnsmessage.NewName(question.Name.String())
	if err != nil {
		return nil, err
	}
	switch question.Type {
	case dnsmessage.TypeA:
		err := builder.AResource(dnsmessage.ResourceHeader{
			Name:  dnsName,
			Class: dnsmessage.ClassINET,
			TTL:   600,
			// 0.0.0.0 - any network interface.
		}, dnsmessage.AResource{A: [4]byte{0, 0, 0, 0}})
		if err != nil {
			return nil, err
		}
	case dnsmessage.TypeAAAA:
		err := builder.AAAAResource(dnsmessage.ResourceHeader{
			Name:  dnsName,
			Class: dnsmessage.ClassINET,
			TTL:   600,
		}, dnsmessage.AAAAResource{
			// ::1 - localhost.
			AAAA: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		})
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("question type must be an address type for building a black hole response")
	}
	return builder.Finish()
}

// DecodeDTMFCommandInput extracts an app command (that may contain DTMF
// sequences) from the input DNS query labels which exclude the domain name.
func DecodeDTMFCommandInput(labels []string) (decodedCommand string) {
	if len(labels) == 0 {
		return ""
	}
	// The first letter of the first label must be the toolbox command prefix, otherwise this cannot possibly be an app command.
	if len(labels[0]) == 0 || labels[0][0] != ToolboxCommandPrefix {
		return ""
	}
	// Remove the toolbox command prefix and continue.
	labels[0] = labels[0][1:]
	// Extract command from remaining eligible labels
	queriedName := strings.Join(labels, "")
	/*
		Most of the special characters and symbols cannot appear in a DNS label, users may still enter them in DTMF
		number sequences. Find all DTMF sequences and translate them back into special characters.
	*/
	consecutiveNumbersRegex := regexp.MustCompile(`[0-9]+`)
	consecutiveNumbers := consecutiveNumbersRegex.FindAllStringSubmatchIndex(queriedName, -1)
	strIdx := 0
	for _, match := range consecutiveNumbers {
		// Collect letters
		if strIdx < match[0] {
			decodedCommand += queriedName[strIdx:match[0]]
		}
		// Decode from DTMF
		decodedCommand += toolbox.DTMFDecode(queriedName[match[0]:match[1]])
		strIdx = match[1]
	}
	// Collect remaining letters
	if strIdx < len(queriedName) {
		decodedCommand += queriedName[strIdx:]
	}
	return
}

// BuildCnameResponse constructs a cname record response for type cname queries.
// The response is not suitable used for A/AAAA queries.
func BuildCnameResponse(header dnsmessage.Header, question dnsmessage.Question, canonicalName string) ([]byte, error) {
	header.Response = true
	header.Truncated = false
	header.Authoritative = true
	header.RecursionAvailable = false
	builder := dnsmessage.NewBuilder(nil, header)
	builder.EnableCompression()
	if err := builder.StartQuestions(); err != nil {
		return nil, err
	}
	if err := builder.Question(question); err != nil {
		return nil, err
	}
	if err := builder.StartAnswers(); err != nil {
		return nil, err
	}
	dnsName, err := dnsmessage.NewName(question.Name.String())
	if err != nil {
		return nil, err
	}
	cname, err := dnsmessage.NewName(canonicalName)
	if err != nil {
		return nil, err
	}
	if builder.CNAMEResource(dnsmessage.ResourceHeader{
		Name:  dnsName,
		Class: dnsmessage.ClassINET,
		TTL:   CommonResponseTTL,
	}, dnsmessage.CNAMEResource{CNAME: cname}) != nil {
		return nil, err
	}
	if err := builder.StartAdditionals(); err != nil {
		return nil, err
	}
	var rh dnsmessage.ResourceHeader
	if err := rh.SetEDNS0(EDNSBufferSize, dnsmessage.RCodeSuccess, false); err != nil {
		return nil, err
	}
	if err := builder.OPTResource(rh, dnsmessage.OPTResource{}); err != nil {
		return nil, err
	}
	return builder.Finish()
}

// BuildSOAResponse returns an SOA record response.
func BuildSOAResponse(header dnsmessage.Header, question dnsmessage.Question, mName, rName string) ([]byte, error) {
	if mName == "" || rName == "" {
		return nil, errors.New("mName and rName must not be empty")
	}
	if mName[len(mName)-1] != '.' {
		mName += "."
	}
	if rName[len(rName)-1] != '.' {
		rName += "."
	}
	rName = strings.Replace(rName, `@`, `.`, -1)
	// Retain the original transaction ID.
	header.Response = true
	header.Truncated = false
	header.Authoritative = true
	header.RecursionAvailable = false
	builder := dnsmessage.NewBuilder(nil, header)
	builder.EnableCompression()
	// Repeat the question back to the client, this is required by DNS protocol.
	if err := builder.StartQuestions(); err != nil {
		return nil, err
	}
	if err := builder.Question(question); err != nil {
		return nil, err
	}
	if err := builder.StartAnswers(); err != nil {
		return nil, err
	}
	dnsMName, err := dnsmessage.NewName(mName)
	if err != nil {
		return nil, err
	}
	dnsRName, err := dnsmessage.NewName(rName)
	if err != nil {
		return nil, err
	}

	serialNum := time.Now().UTC().Format("20060102")
	serialInt, err := strconv.Atoi(serialNum)
	if err != nil {
		return nil, err
	}

	soa := dnsmessage.SOAResource{
		NS:     dnsMName,
		MBox:   dnsRName,
		Serial: uint32(serialInt),
		// "Number of seconds after which secondary name servers should query the master for the SOA record, to detect zone changes." (wikipedia)
		Refresh: 14400,
		// "Number of seconds after which secondary name servers should retry to request the serial number from the master if the master does not respond. It must be less than Refresh." (wikipedia)
		Retry: 3600,
		// "Number of seconds after which secondary name servers should stop answering request for this zone if the master does not respond. This value must be bigger than the sum of Refresh and Retry." (wikipedia)
		Expire: 604800,
		// "Used in calculating the time to live for purposes of negative caching." (wikipedia)
		MinTTL: 300,
	}
	dnsName, err := dnsmessage.NewName(question.Name.String())
	if err != nil {
		return nil, err
	}
	if err := builder.SOAResource(dnsmessage.ResourceHeader{
		Name:  dnsName,
		Class: dnsmessage.ClassINET,
		TTL:   CommonResponseTTL,
	}, soa); err != nil {
		return nil, err
	}
	if err := builder.StartAdditionals(); err != nil {
		return nil, err
	}
	var rh dnsmessage.ResourceHeader
	if err := rh.SetEDNS0(EDNSBufferSize, dnsmessage.RCodeSuccess, false); err != nil {
		return nil, err
	}
	if err := builder.OPTResource(rh, dnsmessage.OPTResource{}); err != nil {
		return nil, err
	}
	return builder.Finish()
}

// BuildMXResponse constructs an MX query response.
func BuildMXResponse(header dnsmessage.Header, question dnsmessage.Question, records []*net.MX) ([]byte, error) {
	if len(records) == 0 {
		return nil, errors.New("mx record(s) must be present")
	}
	// Retain the original transaction ID.
	header.Response = true
	header.Truncated = false
	header.Authoritative = true
	header.RecursionAvailable = false
	builder := dnsmessage.NewBuilder(nil, header)
	builder.EnableCompression()
	// Repeat the question back to the client, this is required by DNS protocol.
	if err := builder.StartQuestions(); err != nil {
		return nil, err
	}
	if err := builder.Question(question); err != nil {
		return nil, err
	}
	if err := builder.StartAnswers(); err != nil {
		return nil, err
	}
	dnsName, err := dnsmessage.NewName(question.Name.String())
	if err != nil {
		return nil, err
	}
	for _, rec := range records {
		mxName, err := dnsmessage.NewName(rec.Host)
		if err != nil {
			return nil, err
		}
		if err := builder.MXResource(dnsmessage.ResourceHeader{
			Name:  dnsName,
			Class: dnsmessage.ClassINET,
			TTL:   CommonResponseTTL,
		}, dnsmessage.MXResource{Pref: rec.Pref, MX: mxName}); err != nil {
			return nil, err
		}
	}
	return builder.Finish()
}

// BuildNSResponse returns an NS record response.
func BuildNSResponse(header dnsmessage.Header, question dnsmessage.Question, domainName string, record NSRecord, glueIP net.IP) ([]byte, error) {
	if domainName == "" {
		return nil, errors.New("domainName must not be empty")
	}
	if domainName[len(domainName)-1] != '.' {
		domainName += "."
	}
	// Retain the original transaction ID.
	header.Response = true
	header.Truncated = false
	header.Authoritative = true
	header.RecursionAvailable = false
	builder := dnsmessage.NewBuilder(nil, header)
	builder.EnableCompression()
	// Repeat the question back to the client, this is required by DNS protocol.
	if err := builder.StartQuestions(); err != nil {
		return nil, err
	}
	if err := builder.Question(question); err != nil {
		return nil, err
	}
	if err := builder.StartAnswers(); err != nil {
		return nil, err
	}
	dnsName, err := dnsmessage.NewName(question.Name.String())
	if err != nil {
		return nil, err
	}
	// Add name servers.
	for _, nsName := range record.Names {
		dnsNSName, err := dnsmessage.NewName(nsName)
		if err != nil {
			return nil, err
		}
		ns := dnsmessage.NSResource{
			NS: dnsNSName,
		}
		if err := builder.NSResource(dnsmessage.ResourceHeader{
			Name:  dnsName,
			Class: dnsmessage.ClassINET,
			TTL:   CommonResponseTTL,
		}, ns); err != nil {
			return nil, err
		}
	}
	// Optional add glue address records into the response.
	// This assumes all NS share the same IPv4 address.
	if !glueIP.Equal(net.IPv4zero) {
		if err := builder.StartAdditionals(); err != nil {
			return nil, err
		}
		v4Addr := glueIP.To4()
		if len(v4Addr) == 4 {
			for _, nsName := range record.Names {
				dnsNSName, err := dnsmessage.NewName(nsName)
				if err != nil {
					return nil, err
				}
				if err := builder.AResource(dnsmessage.ResourceHeader{
					Name:  dnsNSName,
					Class: dnsmessage.ClassINET,
					TTL:   CommonResponseTTL,
				}, dnsmessage.AResource{A: [4]byte{v4Addr[0], v4Addr[1], v4Addr[2], v4Addr[3]}}); err != nil {
					return nil, err
				}
			}
		}
	}
	return builder.Finish()
}

// BuildIPv4AddrResponse constructs an IPv4 address record response. The record
// TTL is hard coded to 60 seconds.
func BuildIPv4AddrResponse(header dnsmessage.Header, question dnsmessage.Question, record V4AddressRecord) ([]byte, error) {
	header.Response = true
	header.Truncated = false
	header.Authoritative = true
	header.RecursionAvailable = false
	builder := dnsmessage.NewBuilder(nil, header)
	builder.EnableCompression()
	if err := builder.StartQuestions(); err != nil {
		return nil, err
	}
	if err := builder.Question(question); err != nil {
		return nil, err
	}
	if err := builder.StartAnswers(); err != nil {
		return nil, err
	}
	dnsName, err := dnsmessage.NewName(question.Name.String())
	if err != nil {
		return nil, err
	}
	if record.CanonicalName == "" {
		for _, ipAddr := range record.Shuffled() {
			v4Addr := ipAddr.To4()
			if v4Addr != nil {
				err := builder.AResource(dnsmessage.ResourceHeader{
					Name:  dnsName,
					Class: dnsmessage.ClassINET,
					TTL:   CommonResponseTTL,
				}, dnsmessage.AResource{A: [4]byte{v4Addr[0], v4Addr[1], v4Addr[2], v4Addr[3]}})
				if err != nil {
					return nil, err
				}
			}
		}
	} else {
		cname, err := dnsmessage.NewName(record.CanonicalName)
		if err != nil {
			return nil, err
		}
		if builder.CNAMEResource(dnsmessage.ResourceHeader{
			Name:  dnsName,
			Class: dnsmessage.ClassINET,
			TTL:   CommonResponseTTL,
		}, dnsmessage.CNAMEResource{CNAME: cname}) != nil {
			return nil, err
		}
	}
	if err := builder.StartAdditionals(); err != nil {
		return nil, err
	}
	var rh dnsmessage.ResourceHeader
	if err := rh.SetEDNS0(EDNSBufferSize, dnsmessage.RCodeSuccess, false); err != nil {
		return nil, err
	}
	if err := builder.OPTResource(rh, dnsmessage.OPTResource{}); err != nil {
		return nil, err
	}
	return builder.Finish()
}

// BuildIPv6AddrResponse constructs an IPv6 address record response. The record
// TTL is hard coded to 60 seconds.
func BuildIPv6AddrResponse(header dnsmessage.Header, question dnsmessage.Question, record V6AddressRecord) ([]byte, error) {
	header.Response = true
	header.Truncated = false
	header.Authoritative = true
	header.RecursionAvailable = false
	builder := dnsmessage.NewBuilder(nil, header)
	builder.EnableCompression()
	if err := builder.StartQuestions(); err != nil {
		return nil, err
	}
	if err := builder.Question(question); err != nil {
		return nil, err
	}
	if err := builder.StartAnswers(); err != nil {
		return nil, err
	}
	dnsName, err := dnsmessage.NewName(question.Name.String())
	if err != nil {
		return nil, err
	}
	if record.CanonicalName == "" {
		for _, ipAddr := range record.Shuffled() {
			if ipAddr.To4() == nil {
				// To16 always returns a non-nil slice for an IPv4 address.
				v6Addr := ipAddr.To16()
				var aaaa [16]byte
				copy(aaaa[:], v6Addr)
				err := builder.AAAAResource(dnsmessage.ResourceHeader{
					Name:  dnsName,
					Class: dnsmessage.ClassINET,
					TTL:   CommonResponseTTL,
				}, dnsmessage.AAAAResource{
					AAAA: aaaa,
				})
				if err != nil {
					return nil, err
				}
			}
		}
	} else {
		cname, err := dnsmessage.NewName(record.CanonicalName)
		if err != nil {
			return nil, err
		}
		if builder.CNAMEResource(dnsmessage.ResourceHeader{
			Name:  dnsName,
			Class: dnsmessage.ClassINET,
			TTL:   CommonResponseTTL,
		}, dnsmessage.CNAMEResource{CNAME: cname}) != nil {
			return nil, err
		}
	}
	if err := builder.StartAdditionals(); err != nil {
		return nil, err
	}
	var rh dnsmessage.ResourceHeader
	if err := rh.SetEDNS0(EDNSBufferSize, dnsmessage.RCodeSuccess, false); err != nil {
		return nil, err
	}
	if err := builder.OPTResource(rh, dnsmessage.OPTResource{}); err != nil {
		return nil, err
	}
	return builder.Finish()
}

// CountNameLabels returns the number of labels in the DNS name.
func CountNameLabels(in string) int {
	in = strings.TrimSpace(in)
	in = strings.TrimPrefix(in, ".")
	in = strings.TrimSuffix(in, ".")
	if in == "" {
		return 0
	}
	return strings.Count(in, ".") + 1
}

// BuildTCPOverDNSSegmentResponse constructs a DNS query response packet that
// encapsulates a TCP-over-DNS segment.
func BuildTCPOverDNSSegmentResponse(header dnsmessage.Header, question dnsmessage.Question, domainName string, seg tcpoverdns.Segment) ([]byte, error) {
	// Retain the original transaction ID.
	header.Response = true
	header.Truncated = false
	header.Authoritative = true
	header.RecursionAvailable = false
	builder := dnsmessage.NewBuilder(nil, header)
	builder.EnableCompression()
	// Repeat the question back to the client, this is required by DNS protocol.
	if err := builder.StartQuestions(); err != nil {
		return nil, err
	}
	if err := builder.Question(question); err != nil {
		return nil, err
	}
	if err := builder.StartAnswers(); err != nil {
		return nil, err
	}
	questionName, err := dnsmessage.NewName(question.Name.String())
	if err != nil {
		return nil, err
	}
	switch question.Type {
	case dnsmessage.TypeCNAME:
		fallthrough
	case dnsmessage.TypeA:
		respSegCname, err := dnsmessage.NewName(seg.DNSName("r", domainName))
		if err != nil {
			return nil, err
		}
		// The first answer RR is a CNAME ("r.data-data-data.example.com") that
		// carries the segment data.
		if err := builder.CNAMEResource(dnsmessage.ResourceHeader{
			Name:  questionName,
			Class: dnsmessage.ClassINET,
			TTL:   CommonResponseTTL,
		}, dnsmessage.CNAMEResource{CNAME: respSegCname}); err != nil {
			return nil, err
		}
		if header.RecursionDesired {
			// If the query asked for an address, then the second RR is a dummy address
			// to the CNAME.
			// There is no useful data in the address.
			switch question.Type {
			case dnsmessage.TypeA:
				err := builder.AResource(dnsmessage.ResourceHeader{
					Name:  respSegCname,
					Class: dnsmessage.ClassINET,
					TTL:   CommonResponseTTL,
				}, dnsmessage.AResource{A: [4]byte{0, 0, 0, 0}})
				if err != nil {
					return nil, err
				}
			case dnsmessage.TypeAAAA:
				err := builder.AAAAResource(dnsmessage.ResourceHeader{
					Name:  respSegCname,
					Class: dnsmessage.ClassINET,
					TTL:   CommonResponseTTL,
				}, dnsmessage.AAAAResource{
					AAAA: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
				})
				if err != nil {
					return nil, err
				}
			}
		}
	case dnsmessage.TypeTXT:
		if err := builder.TXTResource(dnsmessage.ResourceHeader{
			Name:  questionName,
			Class: dnsmessage.ClassINET,
			TTL:   CommonResponseTTL,
		}, dnsmessage.TXTResource{TXT: seg.DNSText()}); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("BuildTCPOverDNSSegmentResponse: unsupported question type %v", question.Type)
	}
	if err := builder.StartAdditionals(); err != nil {
		return nil, err
	}
	var rh dnsmessage.ResourceHeader
	if err := rh.SetEDNS0(EDNSBufferSize, dnsmessage.RCodeSuccess, false); err != nil {
		return nil, err
	}
	if err := builder.OPTResource(rh, dnsmessage.OPTResource{}); err != nil {
		return nil, err
	}
	return builder.Finish()
}
