package dnsd

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

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
	if err := builder.TXTResource(dnsmessage.ResourceHeader{
		Name:  dnsName,
		Class: dnsmessage.ClassINET, TTL: CommonResponseTTL}, dnsmessage.TXTResource{TXT: txt}); err != nil {
		return nil, err
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
	soa := dnsmessage.SOAResource{
		NS:     dnsMName,
		MBox:   dnsRName,
		Serial: 1,
		// "Number of seconds after which secondary name servers should query the master for the SOA record, to detect zone changes." (wikipedia)
		Refresh: 3600,
		// "Number of seconds after which secondary name servers should retry to request the serial number from the master if the master does not respond. It must be less than Refresh." (wikipedia)
		Retry: 300,
		// "Number of seconds after which secondary name servers should stop answering request for this zone if the master does not respond. This value must be bigger than the sum of Refresh and Retry." (wikipedia)
		Expire: 259200,
		// "Used in calculating the time to live for purposes of negative caching." (wikipedia)
		MinTTL: CommonResponseTTL,
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

// BuildMXResponse returns an MX record pointing to the host name.
func BuildMXResponse(header dnsmessage.Header, question dnsmessage.Question, hostName string) ([]byte, error) {
	if len(hostName) == 0 {
		return nil, errors.New("mx host name must not be empty")
	}
	if hostName[len(hostName)-1] != '.' {
		hostName += "."
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
	// The DNS daemon will happily resolve all non-recursive address queries to
	// its own public IP address.
	mxHostName, err := dnsmessage.NewName("mx." + hostName)
	if err != nil {
		return nil, err
	}
	mx := dnsmessage.MXResource{Pref: 10, MX: mxHostName}
	if err := builder.MXResource(dnsmessage.ResourceHeader{
		Name:  dnsName,
		Class: dnsmessage.ClassINET,
		TTL:   CommonResponseTTL,
	}, mx); err != nil {
		return nil, err
	}
	return builder.Finish()
}

// BuildNSResponse returns an NS record response.
func BuildNSResponse(header dnsmessage.Header, question dnsmessage.Question, domainName string, ownIP net.IP) ([]byte, error) {
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
	for i := 1; i <= 4; i++ {
		dnsNSName, err := dnsmessage.NewName(fmt.Sprintf("ns%d.%s", i, domainName))
		if err != nil {
			return nil, err
		}
		// The DNS daemon will happily resolve all non-recursive address queries
		// to its own public IP address.
		ns := dnsmessage.NSResource{
			// ns[1-4].laitos-example.net
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
	if err := builder.StartAdditionals(); err != nil {
		return nil, err
	}
	// Add glue records for the ns[1-4].laitos-example.net.
	v4Addr := ownIP.To4()
	if len(v4Addr) == 4 {
		for i := 1; i <= 4; i++ {
			dnsNSName, err := dnsmessage.NewName(fmt.Sprintf("ns%d.%s", i, domainName))
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
	var rh dnsmessage.ResourceHeader
	if err := rh.SetEDNS0(EDNSBufferSize, dnsmessage.RCodeSuccess, false); err != nil {
		return nil, err
	}
	if err := builder.OPTResource(rh, dnsmessage.OPTResource{}); err != nil {
		return nil, err
	}
	return builder.Finish()
}

// BuildIPv4AddrResponse constructs an IPv4 address record response. The record
// TTL is hard coded to 60 seconds.
func BuildIPv4AddrResponse(header dnsmessage.Header, question dnsmessage.Question, ipAddr net.IP) ([]byte, error) {
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
	switch question.Type {
	case dnsmessage.TypeA:
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
	case dnsmessage.TypeAAAA:
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
