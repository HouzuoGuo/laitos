package dnsd

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/HouzuoGuo/laitos/toolbox"
)

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
	/*
		According to blog post "What is the real maximum length of a DNS name?" authored by "Raymond":
		https://devblogs.microsoft.com/oldnewthing/20120412-00/?p=7873
		The maximum length of a DNS name shall be 254 octets or 253 characters, and each label (e.g. "abc" in abc.example.com)
		may contain up to 63 characters/octets.
		63 characters often aren't long enough for entering a useful command, therefore, look for the command from DNS labels
		connected altogether, minus the domain name that occupies the last 2 labels.
	*/
	if len(queriedName) < 2 || len(queriedName) > 253 || queriedName[0] != ToolboxCommandPrefix {
		return ""
	}
	// Disover labels
	dnsLabels := make([]string, 0)
	for _, label := range strings.Split(queriedName[1:], ".") {
		if trimmedLabel := strings.TrimSpace(label); trimmedLabel != "" {
			dnsLabels = append(dnsLabels, trimmedLabel)
		}
	}
	if len(dnsLabels) < 3 {
		return ""
	}
	// Remove last two DNS labels that belong to domain name
	dnsLabels = dnsLabels[:len(dnsLabels)-2]
	// Extract command from remaining eligible labels
	queriedName = strings.Join(dnsLabels, "")
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
