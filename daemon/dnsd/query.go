package dnsd

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/HouzuoGuo/laitos/toolbox"
)

// QueryPacket encapsulates a DNS query consisting of exactly one question.
type QueryPacket struct {
	TransactionID    []byte
	Flags            []byte
	NumQuestions     int
	NumAnswerRRs     int
	NumAuthorityRRs  int
	NumAdditionalRRs int
	Labels           []string
	Type             []byte
	Class            []byte
}

// IsTextQuery returns true only if the query's only question is looking for a TXT record.
func (pkt *QueryPacket) IsTextQuery() bool {
	// The magic type hex for TXT is 0x0010, class Internet.
	return pkt.NumQuestions == 1 && bytes.Equal(pkt.Type, []byte{0x00, 0x10}) && bytes.Equal(pkt.Class, []byte{0x00, 0x01})

}

// IsTextQuery returns true only if the query's only question is looking for an "A" (IPv4) address record.
func (pkt *QueryPacket) IsNameQuery() bool {
	// The magic type hex for A is 0x0001, class Internet.
	return pkt.NumQuestions == 1 && bytes.Equal(pkt.Type, []byte{0x00, 0x01}) && bytes.Equal(pkt.Class, []byte{0x00, 0x01})
}

// GetHostName returns the host name (e.g. example.com) specified in the query labels.
// The return value preserves the original case presented in the labels.
func (pkt *QueryPacket) GetHostName() string {
	return strings.Join(pkt.Labels, ".")
}

// ParseQueryPacket parses the received DNS (UDP) query packet into individual fields and attributes.
// The function may also be used to parse a DNS-TCP query packet if the caller strips the leading two TCP query length bytes before
// passing the packet into this function.
// If the incoming packet is incomplete, the function will decode as many fields as it can and returns the incomplete
// decoding result.
func ParseQueryPacket(in []byte) (ret *QueryPacket) {
	ret = &QueryPacket{}
	// Because the returned packet structure is going to reference a few slices of the input packet, make a copy of it
	// to prevent the input from being resued for another packet.
	packet := make([]byte, len(in))
	copy(packet, in)
	idx := 0
	// Transaction ID - 2 bytes
	if len(packet) < idx+2 {
		return
	}
	ret.TransactionID = packet[idx : idx+2]
	idx += 2
	// Flags - 2 bytes
	if len(packet) < idx+2 {
		return
	}
	ret.Flags = packet[idx : idx+2]
	idx += 2
	// Number of questions, answer RRs, authority RRs, additional RRs - 2 bytes each
	if len(packet) < idx+2 {
		return
	}
	ret.NumQuestions = int(packet[idx])*256 + int(packet[idx+1])
	idx += 2

	if len(packet) < idx+2 {
		return
	}
	ret.NumAnswerRRs = int(packet[idx])*256 + int(packet[idx+1])
	idx += 2

	if len(packet) < idx+2 {
		return
	}
	ret.NumAuthorityRRs = int(packet[idx])*256 + int(packet[idx+1])
	idx += 2

	if len(packet) < idx+2 {
		return
	}
	ret.NumAdditionalRRs = int(packet[idx])*256 + int(packet[idx+1])
	idx += 2
	// Labels - len(1B), label(lenB), len(1B), label(lenB) ...
	ret.Labels = make([]string, 0, 8)
	for {
		if len(packet) < idx+1 {
			// This is a malformed query packet, was expecting 0x0 to indicate end of labels.
			return
		}
		lenLabel := int(packet[idx])
		idx += 1
		if lenLabel == 0 {
			// This is the end of all labels
			break
		}
		if len(packet) < idx+lenLabel {
			// Malformed packet, was expecting the label text.
			return
		}
		ret.Labels = append(ret.Labels, string(packet[idx:idx+lenLabel]))
		idx += lenLabel
	}
	// Type - 2 bytes
	if len(packet) < idx+2 {
		return
	}
	ret.Type = packet[idx : idx+2]
	idx += 2
	// Class - 2 bytes
	if len(packet) < idx+2 {
		return
	}
	ret.Class = packet[idx : idx+2]
	idx += 2
	return
}

var (
	// standardResponseNoError is a magic flag used in a DNS response packet, it means standard response without an error.
	standardResponseNoError = []byte{129, 128}

	// blackHoleAnswer is an answer to a DNS name query, the answer points the domain in question to "0.0.0.0".
	//                       Domain      A    IN      TTL 1466  IPv4     0.0.0.0
	blackHoleAnswer = []byte{192, 12, 0, 1, 0, 1, 0, 0, 5, 186, 0, 4, 0, 0, 0, 0}

	// textQueryMagic is a series of bytes that appears at the very end of a TXT query question.
	textQueryMagic = []byte{0, 16, 0, 1}
)

// GetBlackHoleResponse returns a DNS response packet (without prefix length bytes) that points queried name to 0.0.0.0.
func GetBlackHoleResponse(queryNoLength []byte) []byte {
	if queryNoLength == nil || len(queryNoLength) < MinNameQuerySize {
		return []byte{}
	}
	answerPacket := make([]byte, 2+2+len(queryNoLength)-4+len(blackHoleAnswer))
	// Match transaction ID of original query
	answerPacket[0] = queryNoLength[0]
	answerPacket[1] = queryNoLength[1]
	// 0x8180 - response is a standard query response, without indication of error.
	copy(answerPacket[2:4], standardResponseNoError)
	// Copy of original query structure
	copy(answerPacket[4:], queryNoLength[4:])
	// There is exactly one answer RR
	answerPacket[6] = 0
	answerPacket[7] = 1
	// Answer 0.0.0.0 to the query
	copy(answerPacket[len(answerPacket)-len(blackHoleAnswer):], blackHoleAnswer)
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
	copy(answerPacket[2:4], standardResponseNoError)
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
