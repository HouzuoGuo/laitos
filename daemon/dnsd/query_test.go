package dnsd

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"
)

// Sample queries for composing test cases
var (
	githubComV4TCPQuery []byte
	githubComV4UDPQuery []byte
	googleComV6UDPQuery []byte
)

func init() {
	var err error
	// Prepare two A queries on "github.coM" (note the capital M, hex 4d) for test cases
	githubComV4TCPQuery, err = hex.DecodeString("00274cc7012000010000000000010667697468756203636f4d00000100010000291000000000000000")
	if err != nil {
		panic(err)
	}
	githubComV4UDPQuery, err = hex.DecodeString("e575012000010000000000010667697468756203636f4d00000100010000291000000000000000")
	if err != nil {
		panic(err)
	}
	googleComV6UDPQuery, err = hex.DecodeString(strings.Replace("84 e3 01 20 00 01 00 00 00 00 00 01 06 67 6f 6f 67 6c 65 03 63 6f 6d 00 00 1c 00 01 00 00 29 10 00 00 00 00 00 00 00", " ", "", -1))
	if err != nil {
		panic(err)
	}
}

func TestGetBlackHoleResponse(t *testing.T) {
	if packet := GetBlackHoleResponse(nil, false); len(packet) != 0 {
		t.Fatal(packet)
	}
	if packet := GetBlackHoleResponse([]byte{}, false); len(packet) != 0 {
		t.Fatal(packet)
	}
	match, err := hex.DecodeString("e575818000010001000000010667697468756203636f4d00000100010000291000000000000000c00c00010001000000ff000400000000")
	if err != nil {
		t.Fatal(err)
	}
	if packet := GetBlackHoleResponse(githubComV4UDPQuery, false); !reflect.DeepEqual(packet, match) {
		t.Fatal(hex.EncodeToString(packet))
	}
}

func TestDecodeDTMFCommandInput(t *testing.T) {
	var tests = []struct {
		labels   []string
		expected string
	}{
		{nil, ""},
		{[]string{"_"}, ""},
		{[]string{"example", "com"}, ""},
		{[]string{"_", "example", "com"}, ""},
		// Decode DTMF spaces
		{[]string{"_0", "example", "com"}, " "},
		{[]string{"_0abc", "example", "com"}, " abc"},
		// Decode DTMF numbers
		{[]string{"_a1b2", "example", "com"}, "a0ba"},
		{[]string{"_a1b2c", "example", "com"}, "a0bac"},
		{[]string{"_0a2", "example", "com"}, " aa"},
		{[]string{"_101010", "example", "com"}, "000"},
		// Decode from multiple labels
		{[]string{"_abc", "def", "example", "com"}, "abcdef"},
		{[]string{"_", "abc", "example", "com"}, "abc"},
		{[]string{"_", "11a", "12b", "13c", "example", "com"}, "1a2b3c"},
	}
	for _, test := range tests {
		if decoded := DecodeDTMFCommandInput(test.labels); decoded != test.expected {
			t.Fatalf("Labels: %v, decoded: %s, expected: %s", test.labels, decoded, test.expected)
		}
	}
}

func TestParseQueryPacketV4(t *testing.T) {
	if ret := ParseQueryPacket(nil); ret.TransactionID != nil || ret.GetNameQueryVersion() != 0 {
		t.Fatal(ret, ret.GetNameQueryVersion())
	}
	if ret := ParseQueryPacket([]byte{0x1, 0x2}); !bytes.Equal(ret.TransactionID, []byte{0x1, 0x2}) || ret.GetNameQueryVersion() != 0 {
		t.Fatal(ret, ret.GetNameQueryVersion())
	}
	gotPacket := ParseQueryPacket(githubComV4UDPQuery)
	wantPacket := &QueryPacket{
		TransactionID:    []byte{0xe5, 0x75},
		Flags:            []byte{0x01, 0x20},
		NumQuestions:     1,
		NumAnswerRRs:     0,
		NumAuthorityRRs:  0,
		NumAdditionalRRs: 1,
		Labels:           []string{"github", "coM"},
		Type:             []byte{0x00, 0x01},
		Class:            []byte{0x00, 0x01},
	}
	if !reflect.DeepEqual(gotPacket, wantPacket) {
		t.Fatalf("\ngot:\n%v\nwant:\n%v\n", gotPacket, wantPacket)
	}
	if hostName := gotPacket.GetHostName(); hostName != "github.coM" {
		t.Fatal(hostName)
	}
	if gotPacket.GetNameQueryVersion() != 4 || gotPacket.IsTextQuery() {
		t.Fatal(gotPacket.GetNameQueryVersion(), gotPacket.IsTextQuery())
	}
}

func TestParseQueryPacketV6(t *testing.T) {
	gotPacket := ParseQueryPacket(googleComV6UDPQuery)
	wantPacket := &QueryPacket{
		TransactionID:    []byte{0x84, 0xe3},
		Flags:            []byte{0x01, 0x20},
		NumQuestions:     1,
		NumAnswerRRs:     0,
		NumAuthorityRRs:  0,
		NumAdditionalRRs: 1,
		Labels:           []string{"google", "com"},
		Type:             []byte{0x00, 0x1c},
		Class:            []byte{0x00, 0x01},
	}
	if !reflect.DeepEqual(gotPacket, wantPacket) {
		t.Fatalf("\ngot:\n%v\nwant:\n%v\n", gotPacket, wantPacket)
	}
	if hostName := gotPacket.GetHostName(); hostName != "google.com" {
		t.Fatal(hostName)
	}
	if gotPacket.GetNameQueryVersion() != 6 || gotPacket.IsTextQuery() {
		t.Fatal(gotPacket.GetNameQueryVersion(), gotPacket.IsTextQuery())
	}
}

func TestParseQueryPacketFuzzy(t *testing.T) {
	for i := 0; i < 20000; i++ {
		malformedPacket := make([]byte, 12345)
		_, err := rand.Reader.Read(malformedPacket)
		if err != nil {
			t.Fatal(err)
		}
		if gotPacket := ParseQueryPacket(malformedPacket); gotPacket.GetNameQueryVersion() != 0 || gotPacket.IsTextQuery() {
			t.Fatalf("Packet: %v, is name query: %v, is text query: %v", malformedPacket, gotPacket.GetNameQueryVersion(), gotPacket.IsTextQuery())
		}
	}
}
