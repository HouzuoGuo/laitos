package dnsd

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"
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

func TestGetBlackHoleResponse(t *testing.T) {
	if packet := GetBlackHoleResponse(nil); len(packet) != 0 {
		t.Fatal(packet)
	}
	if packet := GetBlackHoleResponse([]byte{}); len(packet) != 0 {
		t.Fatal(packet)
	}
	match, err := hex.DecodeString("e575818000010001000000010667697468756203636f4d00000100010000291000000000000000c00c00010001000005ba000400000000")
	if err != nil {
		t.Fatal(err)
	}
	if packet := GetBlackHoleResponse(githubComUDPQuery); !reflect.DeepEqual(packet, match) {
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

func TestParseQueryPacket(t *testing.T) {
	if ret := ParseQueryPacket(nil); ret.TransactionID != nil {
		t.Fatal(ret)
	}
	if ret := ParseQueryPacket([]byte{0x1, 0x2}); !bytes.Equal(ret.TransactionID, []byte{0x1, 0x2}) {
		t.Fatal(ret)
	}
	githubRet := ParseQueryPacket(githubComUDPQuery)
	if !bytes.Equal(githubRet.TransactionID, []byte{0xe5, 0x75}) {
		t.Fatalf("%+v", githubRet)
	}
	if !bytes.Equal(githubRet.Flags, []byte{0x01, 0x20}) {
		t.Fatalf("%+v", githubRet)
	}
	if githubRet.NumQuestions != 1 {
		t.Fatalf("%+v", githubRet)
	}
	if githubRet.NumAnswerRRs != 0 {
		t.Fatalf("%+v", githubRet)
	}
	if githubRet.NumAuthorityRRs != 0 {
		t.Fatalf("%+v", githubRet)
	}
	if githubRet.NumAdditionalRRs != 1 {
		t.Fatalf("%+v", githubRet)
	}
	if !reflect.DeepEqual(githubRet.Labels, []string{"github", "coM"}) {
		t.Fatalf("%+v", githubRet)
	}
	if !bytes.Equal(githubRet.Type, []byte{0x00, 0x01}) {
		t.Fatalf("%+v", githubRet)
	}
	if !bytes.Equal(githubRet.Class, []byte{0x00, 0x01}) {
		t.Fatalf("%+v", githubRet)
	}

	if hostName := githubRet.GetHostName(); hostName != "github.coM" {
		t.Fatal(hostName)
	}
	if !githubRet.IsNameQuery() || githubRet.IsTextQuery() {
		t.Fatal(githubRet.IsNameQuery(), githubRet.IsTextQuery())
	}
}
