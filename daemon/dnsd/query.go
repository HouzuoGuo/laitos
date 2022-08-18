package dnsd

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/HouzuoGuo/laitos/toolbox"
	"golang.org/x/net/dns/dnsmessage"
)

const (
	ednsBufferSize = 1232
)

// BuildTextResponse constructs a TXT record response packet, the record TTL is
// hard coded to 30 seconds.
func BuildTextResponse(name string, header dnsmessage.Header, question dnsmessage.Question, txt []string) ([]byte, error) {
	// Retain the original transaction ID.
	header.Response = true
	header.Truncated = false
	header.Authoritative = true
	header.RecursionAvailable = true
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
	err := builder.TXTResource(dnsmessage.ResourceHeader{
		Name:  dnsmessage.MustNewName(name),
		Class: dnsmessage.ClassINET, TTL: 60}, dnsmessage.TXTResource{TXT: txt})
	if err != nil {
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
	header.RecursionAvailable = true
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
	switch question.Type {
	case dnsmessage.TypeA:
		err := builder.AResource(dnsmessage.ResourceHeader{
			Name:  dnsmessage.MustNewName(question.Name.String()),
			Class: dnsmessage.ClassINET,
			TTL:   600,
			// 0.0.0.0 - any network interface.
		}, dnsmessage.AResource{A: [4]byte{0, 0, 0, 0}})
		if err != nil {
			return nil, err
		}
	case dnsmessage.TypeAAAA:
		err := builder.AAAAResource(dnsmessage.ResourceHeader{
			Name:  dnsmessage.MustNewName(question.Name.String()),
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

// TCPOverDNSSegmentResponse constructs a DNS query response packet for the
// address record representation of a single TCP-over-DNS response.
func TCPOverDNSSegmentResponse(header dnsmessage.Header, question dnsmessage.Question, addrs []dnsmessage.AResource) ([]byte, error) {
	// Retain the original transaction ID.
	header.Response = true
	header.Truncated = false
	header.Authoritative = true
	header.RecursionAvailable = true
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
	for _, addr := range addrs {
		builder.AResource(dnsmessage.ResourceHeader{
			Name:  dnsmessage.MustNewName(question.Name.String()),
			Class: dnsmessage.ClassINET,
			TTL:   60,
		}, addr)
	}
	if err := builder.StartAdditionals(); err != nil {
		return nil, err
	}
	var rh dnsmessage.ResourceHeader
	if err := rh.SetEDNS0(ednsBufferSize, dnsmessage.RCodeSuccess, false); err != nil {
		return nil, err
	}
	if err := builder.OPTResource(rh, dnsmessage.OPTResource{}); err != nil {
		return nil, err
	}
	return builder.Finish()
}

// BuildSOAResponse returns an SOA record response.
func BuildSOAResponse(header dnsmessage.Header, question dnsmessage.Question, mName, rName string) ([]byte, error) {
	// Retain the original transaction ID.
	header.Response = true
	header.Truncated = false
	header.Authoritative = true
	header.RecursionAvailable = true
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
	soa := dnsmessage.SOAResource{
		NS:     dnsmessage.MustNewName(mName),
		MBox:   dnsmessage.MustNewName(rName),
		Serial: 1,
		// "Number of seconds after which secondary name servers should query the master for the SOA record, to detect zone changes." (wikipedia)
		Refresh: 3600,
		// "Number of seconds after which secondary name servers should retry to request the serial number from the master if the master does not respond. It must be less than Refresh." (wikipedia)
		Retry: 300,
		// "Number of seconds after which secondary name servers should stop answering request for this zone if the master does not respond. This value must be bigger than the sum of Refresh and Retry." (wikipedia)
		Expire: 259200,
		// "Used in calculating the time to live for purposes of negative caching." (wikipedia)
		MinTTL: 60,
	}
	err := builder.SOAResource(dnsmessage.ResourceHeader{
		Name:  dnsmessage.MustNewName(question.Name.String()),
		Class: dnsmessage.ClassINET,
		TTL:   60,
	}, soa)
	if err != nil {
		return nil, err
	}
	if err := builder.StartAdditionals(); err != nil {
		return nil, err
	}
	var rh dnsmessage.ResourceHeader
	if err := rh.SetEDNS0(ednsBufferSize, dnsmessage.RCodeSuccess, false); err != nil {
		return nil, err
	}
	if err := builder.OPTResource(rh, dnsmessage.OPTResource{}); err != nil {
		return nil, err
	}
	return builder.Finish()
}

// BuildNSResponse returns an NS record response.
func BuildNSResponse(header dnsmessage.Header, question dnsmessage.Question, domainName string) ([]byte, error) {
	// Retain the original transaction ID.
	header.Response = true
	header.Truncated = false
	header.Authoritative = true
	header.RecursionAvailable = true
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
	for i := 1; i <= 2; i++ {
		ns := dnsmessage.NSResource{
			// ns[1-4].laitos-example.net
			NS: dnsmessage.MustNewName(fmt.Sprintf("ns%d.%s.", i, domainName)),
		}
		err := builder.NSResource(dnsmessage.ResourceHeader{
			Name:  dnsmessage.MustNewName(question.Name.String()),
			Class: dnsmessage.ClassINET,
			TTL:   60,
		}, ns)
		if err != nil {
			return nil, err
		}
	}
	if err := builder.StartAdditionals(); err != nil {
		return nil, err
	}
	var rh dnsmessage.ResourceHeader
	if err := rh.SetEDNS0(ednsBufferSize, dnsmessage.RCodeSuccess, false); err != nil {
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
	header.RecursionAvailable = true
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
	switch question.Type {
	case dnsmessage.TypeA:
		v4Addr := ipAddr.To4()
		if v4Addr != nil {
			err := builder.AResource(dnsmessage.ResourceHeader{
				Name:  dnsmessage.MustNewName(question.Name.String()),
				Class: dnsmessage.ClassINET,
				TTL:   60,
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
				Name:  dnsmessage.MustNewName(question.Name.String()),
				Class: dnsmessage.ClassINET,
				TTL:   60,
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
	if err := rh.SetEDNS0(ednsBufferSize, dnsmessage.RCodeSuccess, false); err != nil {
		return nil, err
	}
	if err := builder.OPTResource(rh, dnsmessage.OPTResource{}); err != nil {
		return nil, err
	}
	return builder.Finish()
}
