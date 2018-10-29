package snmp

import (
	"bufio"
	"bytes"
	"encoding/asn1"
	"fmt"
	"reflect"
	"testing"
)

func TestGetNext(t *testing.T) {
	reqBytes := []byte{
		//ASN1  SZ   INT    SZ
		0x30, 0x2a, 0x02, 0x01,
		//v2  OSTR    SZ     p     u    b      l     i     c APDU1    SZ   INT    SZ  REQID460219274...
		0x01, 0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0xa1, 0x1d, 0x02, 0x04, 0x1b, 0x6e, 0x63,
		//..   INT   SZ   NoErr  INT   SZ  EIDX0  ASN1    SZ  ASN1    SZ   OID    SZ  1.3     .6    .1
		0x8a, 0x02, 0x01, 0x00, 0x02, 0x01, 0x00, 0x30, 0x0f, 0x30, 0x0d, 0x06, 0x09, 0x2b, 0x06, 0x01,
		//.2    .1    .1    .1    .1    .0   NUL    SZ
		0x02, 0x01, 0x01, 0x01, 0x01, 0x00, 0x05, 0x00,
	}
	p := Packet{}
	if err := p.ReadFrom(bufio.NewReader(bytes.NewReader(reqBytes))); err != nil {
		t.Fatal(err)
	}
	if p.Version != 1 || p.CommunityName != "public" || p.PDU != PDUGetNextRequest || p.RequestID != 460219274 ||
		p.ErrorIndex != 0 || p.ErrorStatus != 0 {
		t.Fatalf("%+v", p)
	}
	req := p.Structure.(GetNextRequest)
	if !reflect.DeepEqual(req.BaseOID, asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 1, 1, 0}) {
		t.Fatalf("%+v", req)
	}

	respBytes := []byte{
		//ASN1  SZ   INT    SZ
		0x30, 0x33, 0x02, 0x01,
		//v2  OSTR    SZ     p     u    b      l     i     c APDU1    SZ   INT    SZ  REQID460219274...
		0x01, 0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0xa2, 0x26, 0x02, 0x04, 0x1b, 0x6e, 0x63,
		//..   INT   SZ   NoErr  INT   SZ  EIDX0  ASN1    SZ  ASN1    SZ   OID    SZ  1.3     .6    .1
		0x8a, 0x02, 0x01, 0x00, 0x02, 0x01, 0x00, 0x30, 0x18, 0x30, 0x16, 0x06, 0x08, 0x2b, 0x06, 0x01,
		//.2    .1    .1    .2    .0   OID   SZ    1.3    .6    .1    .4    .1   .80____72    .3    .2
		0x02, 0x01, 0x01, 0x02, 0x00, 0x06, 0x0a, 0x2b, 0x06, 0x01, 0x04, 0x01, 0xbf, 0x08, 0x03, 0x02,
		//.10
		0x0a,
	}
	p.PDU = PDUGetResponse
	p.Structure = GetResponse{
		RequestedOID: asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 2, 0},
		Value:        asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 8072, 3, 2, 10},
	}
	encodedResp, err := p.Encode()
	if err != nil || !reflect.DeepEqual(encodedResp, respBytes) {
		t.Fatalf("\n%+v\n%+v\n%+v\n", err, encodedResp, respBytes)
	}
}

func TestGetSpecialResponse(t *testing.T) {
	reqBytes := []byte{
		//ASN1  SZ   INT    SZ
		0x30, 0x2a, 0x02, 0x01,
		//v2  OSTR    SZ     p     u    b      l     i     c APDU0    SZ   INT    SZ  REQID460219275...
		0x01, 0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0xa0, 0x1d, 0x02, 0x04, 0x1b, 0x6e, 0x63,
		//..   INT   SZ   NoErr  INT   SZ  EIDX0  ASN1    SZ  ASN1    SZ   OID    SZ  1.3     .6    .1
		0x8b, 0x02, 0x01, 0x00, 0x02, 0x01, 0x00, 0x30, 0x0f, 0x30, 0x0d, 0x06, 0x09, 0x2b, 0x06, 0x01,
		//.2    .1    .1    .1    .1    .0   NUL    SZ
		0x02, 0x01, 0x01, 0x01, 0x01, 0x00, 0x05, 0x00,
	}
	p := Packet{}
	if err := p.ReadFrom(bufio.NewReader(bytes.NewReader(reqBytes))); err != nil {
		t.Fatal(err)
	}
	if p.Version != 1 || p.CommunityName != "public" || p.PDU != PDUGetRequest || p.RequestID != 460219275 ||
		p.ErrorIndex != 0 || p.ErrorStatus != 0 {
		t.Fatalf("%+v", p)
	}
	req := p.Structure.(GetRequest)
	if !reflect.DeepEqual(req.RequestedOID, asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 1, 1, 0}) {
		t.Fatalf("%+v", req)
	}

	// NoSuchInstance response
	respBytes := []byte{
		//ASN1  SZ   INT    SZ
		0x30, 0x2a, 0x02, 0x01,
		//v2  OSTR    SZ     p     u    b      l     i     c APDU0    SZ   INT    SZ  REQID460219275...
		0x01, 0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0xa2, 0x1d, 0x02, 0x04, 0x1b, 0x6e, 0x63,
		//..   INT   SZ   NoErr  INT   SZ  EIDX0  ASN1    SZ  ASN1    SZ   OID    SZ  1.3     .6    .1
		0x8b, 0x02, 0x01, 0x00, 0x02, 0x01, 0x00, 0x30, 0x0f, 0x30, 0x0d, 0x06, 0x09, 0x2b, 0x06, 0x01,
		//.2    .1    .1    .1    .1    .0  NoSuchInst
		0x02, 0x01, 0x01, 0x01, 0x01, 0x00, 0x81, 0x00,
	}
	p.PDU = PDUGetResponse
	p.Structure = GetResponse{
		RequestedOID:   asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 1, 1, 0},
		NoSuchInstance: true,
	}
	encodedResp, err := p.Encode()
	if err != nil || !reflect.DeepEqual(encodedResp, respBytes) {
		fmt.Printf("\n%+v\n%+v\n%+v\n", err, encodedResp, respBytes)
		t.Fatal()
	}

	// EndOfMibView response
	respBytes = []byte{
		//ASN1  SZ   INT    SZ
		0x30, 0x2a, 0x02, 0x01,
		//v2  OSTR    SZ     p     u    b      l     i     c APDU0    SZ   INT    SZ  REQID460219275...
		0x01, 0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0xa2, 0x1d, 0x02, 0x04, 0x1b, 0x6e, 0x63,
		//..   INT   SZ   NoErr  INT   SZ  EIDX0  ASN1    SZ  ASN1    SZ   OID    SZ  1.3     .6    .1
		0x8b, 0x02, 0x01, 0x00, 0x02, 0x01, 0x00, 0x30, 0x0f, 0x30, 0x0d, 0x06, 0x09, 0x2b, 0x06, 0x01,
		//.2    .1    .1    .1    .1    .0  EndOfMIBView
		0x02, 0x01, 0x01, 0x01, 0x01, 0x00, 0x82, 0x00,
	}
	p.PDU = PDUGetResponse
	p.Structure = GetResponse{
		RequestedOID: asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 1, 1, 0},
		EndOfMIBView: true,
	}
	encodedResp, err = p.Encode()
	if err != nil || !reflect.DeepEqual(encodedResp, respBytes) {
		fmt.Printf("\n%+v\n%+v\n%+v\n", err, encodedResp, respBytes)
		t.Fatal()
	}

}

func TestGet(t *testing.T) {
	reqBytes := []byte{
		//ASN1  SZ   INT    SZ
		0x30, 0x29, 0x02, 0x01,
		//v2  OSTR    SZ     p     u    b      l     i     c APDU1    SZ   INT    SZ  REQID‭1501346825‬..
		0x01, 0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0xa1, 0x1c, 0x02, 0x04, 0x59, 0x7c, 0xbc,
		//..   INT   SZ   NoErr  INT   SZ  EIDX0  ASN1    SZ  ASN1    SZ   OID    SZ  1.3     .6    .1
		0x09, 0x02, 0x01, 0x00, 0x02, 0x01, 0x00, 0x30, 0x0e, 0x30, 0x0c, 0x06, 0x08, 0x2b, 0x06, 0x01,
		//.2    .1    .1    .3    .0   NUL    SZ
		0x02, 0x01, 0x01, 0x03, 0x00, 0x05, 0x00,
	}
	p := Packet{}
	if err := p.ReadFrom(bufio.NewReader(bytes.NewReader(reqBytes))); err != nil {
		t.Fatal(err)
	}
	if p.Version != 1 || p.CommunityName != "public" || p.PDU != PDUGetNextRequest || p.RequestID != 1501346825 ||
		p.ErrorIndex != 0 || p.ErrorStatus != 0 {
		t.Fatalf("%+v", p)
	}
	req := p.Structure.(GetNextRequest)
	if !reflect.DeepEqual(req.BaseOID, asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 3, 0}) {
		t.Fatalf("%+v", req)
	}

	respBytes := []byte{
		//ASN1  SZ   INT    SZ
		0x30, 0x3c, 0x02, 0x01,
		//v2  OSTR    SZ     p     u    b      l     i     c APDU1    SZ   INT    SZ  REQID‭1501346825‬..
		0x01, 0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0xa2, 0x2f, 0x02, 0x04, 0x59, 0x7c, 0xbc,
		//..   INT   SZ   NoErr  INT   SZ  EIDX0  ASN1    SZ  ASN1    SZ   OID    SZ  1.3     .6    .1
		0x09, 0x02, 0x01, 0x00, 0x02, 0x01, 0x00, 0x30, 0x21, 0x30, 0x1f, 0x06, 0x08, 0x2b, 0x06, 0x01,
		//.2    .1    .1    .4    .0  OSTR    SZ     M     e  <SPC>    <     m     e     @     e     x
		0x02, 0x01, 0x01, 0x04, 0x00, 0x04, 0x13, 0x4d, 0x65, 0x20, 0x3c, 0x6d, 0x65, 0x40, 0x65, 0x78,
		// a     m     p     l     e     .     o     r     g     >
		0x61, 0x6d, 0x70, 0x6c, 0x65, 0x2e, 0x6f, 0x72, 0x67, 0x3e,
	}
	p.PDU = PDUGetResponse
	p.Structure = GetResponse{
		RequestedOID: asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 4, 0},
		Value:        []byte("Me <me@example.org>"),
	}
	encodedResp, err := p.Encode()
	if err != nil || !reflect.DeepEqual(encodedResp, respBytes) {
		fmt.Printf("\n%+v\n%+v\n%+v\n", err, encodedResp, respBytes)
		t.Fatal()
	}
}

func TestASN(t *testing.T) {
	t.Log(asn1.Marshal([]byte("abc")))
}
