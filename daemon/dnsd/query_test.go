package dnsd

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"testing"
)

func TestExtractTextQueryName(t *testing.T) {
	// Prepare two TXT queries on (verysecret.e a) "_88833777999777733222777338014203322244666002.hz.gl"
	cmdTextTCPQuery, err := hex.DecodeString("0056d21e01200001000000000001335f383838333337373739393937373737333332323237373733333830313432303737373730303333323232343436363630303202687a02676c00001000010000291000000000000000")
	if err != nil {
		panic(err)
	}
	cmdTextUDPQuery, err := hex.DecodeString("a91701200001000000000001335f383838333337373739393937373737333332323237373733333830313432303737373730303333323232343436363630303202687a02676c00001000010000291000000000000000")
	if err != nil {
		panic(err)
	}

	/*
		sampleCommandDTMF is the DTMF input of:
								   v e  r  y   s e  c  r et   .    s  e  c h  o  a"
	*/
	var sampleCommandDTMF = "88833777999777733222777338014207777003322244666002"

	// TCP query length field is two bytes long
	if queriedName := ExtractTextQueryInput(cmdTextTCPQuery[2:]); queriedName != fmt.Sprintf("_%s.hz.gl", sampleCommandDTMF) {
		t.Fatalf("\n%+v\n%+v\n", sampleCommandDTMF, queriedName)
	} else if cmd := DecodeDTMFCommandInput(queriedName); cmd != "verysecret.s echo a" {
		t.Fatal(cmd)
	}
	if queriedName := ExtractTextQueryInput(cmdTextUDPQuery); queriedName != fmt.Sprintf("_%s.hz.gl", sampleCommandDTMF) {
		t.Fatalf("\n%+v\n%+v\n", sampleCommandDTMF, queriedName)
	} else if cmd := DecodeDTMFCommandInput(queriedName); cmd != "verysecret.s echo a" {
		t.Fatal(cmd)
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

func TestDecodeDTMFCommandInput(t *testing.T) {
	if d := DecodeDTMFCommandInput(""); d != "" {
		t.Fatal(d)
	}
	if d := DecodeDTMFCommandInput("_"); d != "" {
		t.Fatal(d)
	}
	// 0 -> space
	if d := DecodeDTMFCommandInput("_0"); d != " " {
		t.Fatal(d)
	}
	if d := DecodeDTMFCommandInput("123"); d != "" {
		t.Fatal(d)
	}
	if d := DecodeDTMFCommandInput("abc"); d != "" {
		t.Fatal(d)
	}
	if d := DecodeDTMFCommandInput("_abc"); d != "abc" {
		t.Fatal(d)
	}
	// 0 -> 1, 2 -> a
	if d := DecodeDTMFCommandInput("_a1b2"); d != "a0ba" {
		t.Fatal(d)
	}
	if d := DecodeDTMFCommandInput("_a1b2c"); d != "a0bac" {
		t.Fatal(d)
	}
	// 0 -> space, 2 -> a
	if d := DecodeDTMFCommandInput("_0a2"); d != " aa" {
		t.Fatal(d)
	}
	// 10 -> number 0 literally
	if d := DecodeDTMFCommandInput("_101010"); d != "000" {
		t.Fatal(d)
	}
	if d := DecodeDTMFCommandInput("_101010.a.b"); d != "000" {
		t.Fatal(d)
	}

}
