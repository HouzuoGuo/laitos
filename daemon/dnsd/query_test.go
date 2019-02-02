package dnsd

import (
	"encoding/hex"
	"reflect"
	"testing"
)

func TestExtractTextQueryName(t *testing.T) {
	// TCP query length field is two bytes long
	if extracted := ExtractTextQueryInput(cmdTextTCPQuery[2:]); extracted != cmdTextRestored {
		t.Fatalf("\n%+v\n%+v\n", extracted, cmdTextRestored)
	}
	if extracted := ExtractTextQueryInput(cmdTextUDPQuery); extracted != cmdTextRestored {
		t.Fatalf("\n%+v\n%+v\n", extracted, cmdTextRestored)
	}
}

func TestExtractDomainName(t *testing.T) {
	if name := ExtractDomainName(nil); name != "" {
		t.Fatal(name)
	}
	if name := ExtractDomainName([]byte{}); name != "" {
		t.Fatal(name)
	}
	if name := ExtractDomainName(githubComUDPQuery); name != "github.coM" {
		t.Fatal(name)
	}
	// TCP query length field is two bytes long
	if name := ExtractDomainName(githubComTCPQuery[2:]); name != "github.coM" {
		t.Fatal(name)
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
