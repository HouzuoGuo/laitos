package dnsd

import (
	"bytes"
	"encoding/hex"
	"strings"
)

// Sample queries for composing test cases
var githubComTCPQuery []byte
var githubComUDPQuery []byte
var cmdTextRestored = "verysecret .s echo '0001~!@#$%^&*()_+-={}[]:\"|;\\<>?,/~`'"
var cmdTextTCPQuery []byte
var cmdTextUDPQuery []byte

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
	/*
		Prepare a TXT query toolbox command for test cases:
		a=$'verysecret 00s echo \'00000101~!@#$%^&*()_+-={}[]:"|;01<>?,/~`\''
		dig -t TXT "$a" +tcp
		dig -t TXT "$a"
		dig -t TXT hz.gl +tcp
		dig -t TXT hz.gl
		Where 00 translates into a full-stop and "verysecret" is the value of TestCommandProcessorPIN.
		Due to various quirks of DNS protocol, the length of the query above is equal or very close to the maximum
		accepted by popular DNS clients.
	*/
	cmdTextTCPQuery, err = hex.DecodeString("005bf31c012000010000000000013e7665727973656372657420303073206563686f202730303030303130317e21402324255e262a28295f2b2d3d7b7d5b5d3a227c3b30313c3e3f2c2f7e602700001000010000291000000000000000")
	if err != nil {
		panic(err)
	}
	cmdTextUDPQuery, err = hex.DecodeString("6361012000010000000000013e7665727973656372657420303073206563686f202730303030303130317e21402324255e262a28295f2b2d3d7b7d5b5d3a227c3b30313c3e3f2c2f7e602700001000010000291000000000000000")
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
	answerPacket := make([]byte, len(queryNoLength))
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
	// Answer entry magic c0 0c
	answerPacket = append(answerPacket, 0xc0, 0x0c)
	// Text type, Class IN
	answerPacket = append(answerPacket, textQueryMagic...)
	// TTL - 30 seconds (the minimum acceptable TTL by consensus, not by standard)
	answerPacket = append(answerPacket, 0x0, 0x0, 0x0, 0x1e)
	// Data length (2 bytes) = TXT length + 1
	answerPacket = append(answerPacket, 0x0, byte(len(text)+1))
	// TXT length = length of input text
	answerPacket = append(answerPacket, byte(len(text)))
	// At last there is the text entry content
	answerPacket = append(answerPacket, []byte(text)...)
	return answerPacket
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

/*
ExtractDomainName extracts domain name or toolbox command input requested by input query packet. If the function fails
to identify them, it will return an empty string.
*/
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
	/*
		The routine used in ExtractDomainName that translates certain byte ranges into full-stop is not used here, and
		DNS imposes couple of other restrictions on the set of characters valid for a query. To work around those
		restrictions:
		-- Text 0000 translates into 00
		-- Text 00 translates into full-stop (.)
		-- Text 0101 translates into 01
		-- Text 01 translates into back-slash (\)
		Letters, numbers, space, and other symbols come through just fine.
	*/
	// "0000" -> 0x0  "00" -> "."  0x0 -> "00"
	queriedNameBytes = bytes.Replace(queriedNameBytes, []byte{48, 48, 48, 48}, []byte{0}, -1)
	queriedNameBytes = bytes.Replace(queriedNameBytes, []byte{48, 48}, []byte{46}, -1)
	queriedNameBytes = bytes.Replace(queriedNameBytes, []byte{0}, []byte{48, 48}, -1)
	// "0101" -> 0x1  "01" -> "\"  0x1 -> "01"
	queriedNameBytes = bytes.Replace(queriedNameBytes, []byte{48, 49, 48, 49}, []byte{1}, -1)
	queriedNameBytes = bytes.Replace(queriedNameBytes, []byte{48, 49}, []byte{92}, -1)
	queriedNameBytes = bytes.Replace(queriedNameBytes, []byte{1}, []byte{48, 49}, -1)
	queriedName := strings.TrimSpace(string(queriedNameBytes))
	// Do not extract domain name that is exceedingly long
	if len(queriedName) > 255 {
		return ""
	}
	return queriedName
}
