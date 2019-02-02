package dnsd

import (
	"bytes"
	"encoding/hex"
	"strings"
)

var githubComTCPQuery []byte
var githubComUDPQuery []byte // Sample queries for composing test cases

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
	// Finally, respond!
	return answerPacket
}

/*
ExtractDomainName extracts domain name requested by input query packet. If the function fails to determine the requested
name, it returns an empty string.
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
	/*
		Restore full-stops of the domain name portion so that it can be checked against black list.
		Not sure why those byte ranges show up in place of full-stops, probably due to some RFCs.
	*/
	for i, b := range domainNameBytes {
		if b <= 44 || b >= 58 && b <= 64 || b >= 91 && b <= 96 {
			domainNameBytes[i] = '.'
		}
	}
	domainName := strings.TrimSpace(string(domainNameBytes))
	// Do not extract domain name that is exceedingly long
	if len(domainName) > 255 {
		return ""
	}
	return domainName
}
