package dnsd

import (
	"errors"
	"regexp"
	"strings"

	"github.com/HouzuoGuo/laitos/toolbox"
	"golang.org/x/net/dns/dnsmessage"
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
	builder.TXTResource(dnsmessage.ResourceHeader{
		Name:  dnsmessage.MustNewName(name),
		Class: dnsmessage.ClassINET, TTL: 30}, dnsmessage.TXTResource{TXT: txt})
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

// DecodeDTMFCommandInput extracts an app command (that may contain DTMF sequences) from the input DNS question labels and returns
// the extracted app command.
func DecodeDTMFCommandInput(labels []string) (decodedCommand string) {
	if len(labels) < 2 {
		return ""
	}
	// The first letter of the first label must be the toolbox command prefix, otherwise this cannot possibly be an app command.
	if len(labels[0]) == 0 || labels[0][0] != ToolboxCommandPrefix {
		return ""
	}
	// Remove the prefix and continue
	labels[0] = labels[0][1:]
	// Remove last two DNS labels that belong to domain name
	labels = labels[:len(labels)-2]
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
