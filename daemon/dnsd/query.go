package dnsd

import (
	"bytes"
	"encoding/hex"
	"github.com/HouzuoGuo/laitos/daemon/httpd/handler"
	"regexp"
	"strings"
)

// Sample queries for composing test cases
var githubComTCPQuery []byte
var githubComUDPQuery []byte

func init() {
	var err error
	// Prepare two A queries on "github.coM" (note the capital M, hex 4d) for test cases
	githubComTCPQuery, err = hex.DecodeString("00274cc7012000010000000000010667697468756203636f4d00000100010000291000000000000000")
	if err != nil {
		panic(err)
	}
	githubComUDPQuery, err = hex.DecodeString("e575012000010000000000010667697468756203636f4d00000100010000291000000000000000")
	if err != nil {
		panic(err)
	}
}

var StandardResponseNoError = []byte{129, 128} // DNS response packet flag - standard response, no indication of error.

//                            Domain     A    IN      TTL 1466  IPv4     0.0.0.0
var BlackHoleAnswer = []byte{192, 12, 0, 1, 0, 1, 0, 0, 5, 186, 0, 4, 0, 0, 0, 0} // DNS answer 0.0.0.0

// GetBlackHoleResponse returns a DNS response packet (without prefix length bytes) that points queried name to 0.0.0.0.
func GetBlackHoleResponse(queryNoLength []byte) []byte {
	if queryNoLength == nil || len(queryNoLength) < MinNameQuerySize {
		return []byte{}
	}
	answerPacket := make([]byte, 2+2+len(queryNoLength)-4+len(BlackHoleAnswer))
	// Match transaction ID of original query
	answerPacket[0] = queryNoLength[0]
	answerPacket[1] = queryNoLength[1]
	// 0x8180 - response is a standard query response, without indication of error.
	copy(answerPacket[2:4], StandardResponseNoError)
	// Copy of original query structure
	copy(answerPacket[4:], queryNoLength[4:])
	// There is exactly one answer RR
	answerPacket[6] = 0
	answerPacket[7] = 1
	// Answer 0.0.0.0 to the query
	copy(answerPacket[len(answerPacket)-len(BlackHoleAnswer):], BlackHoleAnswer)
	return answerPacket
}

func MakeTextResponse(queryNoLength []byte, text string) []byte {
	if queryNoLength == nil || len(queryNoLength) < MinNameQuerySize {
		return []byte{}
	}
	// Limit response to 254 characters maximum, I am feeling lazy to implement multi-entry reply.
	if len(text) > 254 {
		text = text[:254]
	}

	queryMagicIndex := bytes.Index(queryNoLength[MinNameQuerySize:], textQueryMagic)
	if queryMagicIndex < 0 {
		return []byte{}
	}
	// Copy input packet into output packet
	answerPacket := make([]byte, 0, len(queryNoLength))
	answerPacket = append(answerPacket, queryNoLength[:MinNameQuerySize+queryMagicIndex+len(textQueryMagic)]...)

	// Manipulate response based on the copied input query
	// Byte 0, 1 - transaction ID already matches that of input query
	// Byte 2, 3 - standard response, no error.
	copy(answerPacket[2:4], StandardResponseNoError)
	// Byte 6, 7 - there is exactly one answer RR
	answerPacket[6] = 0
	answerPacket[7] = 1

	// Answer entry magic c0 0c
	answerPacket = append(answerPacket, 0xc0, 0x0c)
	// Text type, Class IN
	answerPacket = append(answerPacket, textQueryMagic...)
	// TTL - 30 seconds (the minimum acceptable TTL by consensus, not by standard)
	answerPacket = append(answerPacket, 0x0, 0x0, 0x0, TextCommandReplyTTL)
	// Data length (2 bytes) = TXT length + 1
	answerPacket = append(answerPacket, 0x0, byte(len(text)+1))
	// TXT length = length of input text
	answerPacket = append(answerPacket, byte(len(text)))
	// Text entry content
	answerPacket = append(answerPacket, []byte(text)...)
	// Additional Record from the original packet
	queryAdditionalRecord := queryNoLength[queryMagicIndex+MinNameQuerySize:]
	answerPacket = append(answerPacket, queryAdditionalRecord...)

	return answerPacket
}

/*
lintQueriedDomainName modifies input domain name in-place to recover full-stop symbols that somehow came as bytes not
in the range of readable characters.
*/
func recoverFullStopSymbols(in []byte) {
	// This is perhaps a quirk of some DNS-related RFC
	for i, b := range in {
		if b <= 44 || b >= 58 && b <= 64 || b >= 91 && b <= 96 && b != 95 {
			in[i] = '.'
		}
	}
}

/*
ExtractDomainName extracts domain name requested by input query packet. If the function fails to identify a domain name,
it will return an empty string.
*/
func ExtractDomainName(packet []byte) string {
	if packet == nil || len(packet) < MinNameQuerySize {
		return ""
	}
	indexTypeAClassIN := bytes.Index(packet[13:], nameQueryMagic)
	if indexTypeAClassIN < 1 {
		return ""
	}
	indexTypeAClassIN += 13
	// The byte right before Type-A Class-IN is an empty byte to be discarded
	domainNameBytes := make([]byte, indexTypeAClassIN-13-1)
	copy(domainNameBytes, packet[13:indexTypeAClassIN-1])
	recoverFullStopSymbols(domainNameBytes)
	domainName := strings.TrimSpace(string(domainNameBytes))
	// Do not extract domain name that is exceedingly long
	if len(domainName) > 255 {
		return ""
	}
	return domainName
}

// ExtractTextQueryInput extracts queried name from a TXT query packet.
func ExtractTextQueryInput(packet []byte) string {
	if packet == nil || len(packet) < MinNameQuerySize {
		return ""
	}
	indexTypeTXTClassIN := bytes.Index(packet[13:], textQueryMagic)
	if indexTypeTXTClassIN < 1 {
		return ""
	}
	indexTypeTXTClassIN += 13
	// The byte right before Type-A Class-IN is an empty byte to be discarded
	queriedNameBytes := make([]byte, indexTypeTXTClassIN-13-1)
	copy(queriedNameBytes, packet[13:indexTypeTXTClassIN-1])
	// Do not extract domain name that is exceedingly long
	if len(queriedNameBytes) > 255 {
		return ""
	}
	recoverFullStopSymbols(queriedNameBytes)
	return string(queriedNameBytes)
}

/*
DecodeDTMFCommandInput decodes input query name consisting of latin letter input and DTMF sequences, and returns the
complete, recovered toolbox command input.
*/
func DecodeDTMFCommandInput(queriedName string) (decodedCommand string) {
	if len(queriedName) < 2 {
		return ""
	}
	if queriedName[0] != ToolboxCommandPrefix {
		return ""
	}
	// Extract command input from the sub-domain name only
	indexDot := strings.IndexRune(queriedName, '.')
	if indexDot == -1 {
		// In case the query does not look like a domain name at all, try interpreting the entire sequence as a command.
		indexDot = len(queriedName)
	}
	// Skip the prefix and parts beyond sub-domain name
	queriedName = queriedName[1:indexDot]

	// Look for DTMF number sequences
	consecutiveNumbersRegex := regexp.MustCompile(`[0-9]+`)
	consecutiveNumbers := consecutiveNumbersRegex.FindAllStringSubmatchIndex(queriedName, -1)
	strIdx := 0
	for _, match := range consecutiveNumbers {
		// Collect letters
		if strIdx < match[0] {
			decodedCommand += queriedName[strIdx:match[0]]
		}
		// Decode from DTMF
		decodedCommand += handler.DTMFDecode(queriedName[match[0]:match[1]])
		strIdx = match[1]
	}
	// Collect remaining letters
	if strIdx < len(queriedName) {
		decodedCommand += queriedName[strIdx:]
	}
	return
}
