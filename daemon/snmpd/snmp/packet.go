/*
snmp implements a rudimentary encoder and decoder of SNMP packets. It understands GetNextRequest, GetRequest, and
GetResponse.
*/
package snmp

import (
	"bufio"
	"encoding/asn1"
	"errors"
	"fmt"
)

func MissedExpectation(kind, expected, actual interface{}) error {
	if expected == nil {
		return fmt.Errorf("unexpected %v %v", kind, actual)
	}
	return fmt.Errorf("unexpected %v %v (actual is %v)", kind, actual, expected)
}

const (
	// TagASN1 is the magic tag corresponding to ASN.1 data type in SNMP packet.
	TagASN1 = 0x30
	// TagNoSuchInstance is the magic tag corresponding to a response made toward non-existing OID.
	TagNoSuchInstance = 0x81
	// TagEndOfMIBView is the magic tag corresponding to a response made toward one OID beyond the last one in hierarchy.
	TagEndOfMIBView = 0x82
	// ProtocolV2C is the protocol version magic corresponding to SNMP version 2.0 with community name.
	ProtocolV2C = 0x01

	// PDUGetNextRequest asks for the subsequent OID in a walk operation.
	PDUGetNextRequest = 0xa1
	// PDUGetResponse is a response to either Get or GetNext request.
	PDUGetResponse = 0xa2
	// PDUGetRequest asks for value corresponding to an OID.
	PDUGetRequest = 0xa0
)

// ReadTag returns a primitive tag read from input.
func ReadTag(in *bufio.Reader) (tag byte, err error) {
	tag, err = in.ReadByte()
	if tag < 0 {
		err = MissedExpectation("tag", nil, tag)
	}
	return
}

// ReadTag returns a primitive object size read from input.
func ReadSize(in *bufio.Reader) (size byte, err error) {
	size, err = in.ReadByte()
	if size < 0 {
		err = MissedExpectation("size", nil, size)
	}
	return
}

// ReadTag returns a primitive tag and primitive object size read from input.
func ReadTagSize(in *bufio.Reader) (tag byte, size byte, err error) {
	tag, err = ReadTag(in)
	if err != nil {
		return
	}
	size, err = ReadSize(in)
	return
}

// ReadBytes returns consecutive bytes read from input.
func ReadBytes(in *bufio.Reader, n int) (bytes []byte, err error) {
	bytes = make([]byte, n)
	_, err = in.Read(bytes)
	return
}

// UnmarshalInteger reads a primitive integer and advances the slice.
func UnmarshalInteger(in *[]byte) (i int64, err error) {
	rest, err := asn1.Unmarshal(*in, &i)
	*in = rest
	return
}

// UnmarshalInteger reads a primitive string and advances the slice.
func UnmarshalString(in *[]byte) (s string, err error) {
	var sBytes []byte
	rest, err := asn1.Unmarshal(*in, &sBytes)
	s = string(sBytes)
	*in = rest
	return
}

// Packet describes a complete SNMP packet, no matter it is a request or a response.
type Packet struct {
	Version       int64  // Version is the SNMP version, laitos only supports version 2 (value 1).
	CommunityName string // CommunityName is a password string communicated in plain text.
	PDU           byte   // PDU determines the type of SNMP request or response described in a packet.
	RequestID     int64  // RequestID is an integer shared by pairs of request and response packets.
	ErrorStatus   int64  // ErrorStatus is an SNMP magic not handled by laitos.
	ErrorIndex    int64  // ErrorIndex is an SNMP magic not handled by laitos.

	Structure interface{} // Structure contains details of an SNMP PDU request or response.
}

// ReadPacket deserialises an SNMP packet from input.
func (packet *Packet) ReadFrom(in *bufio.Reader) (err error) {
	// Read ASN1 tag and packet size
	tag, size, err := ReadTagSize(in)
	if err != nil {
		return
	}
	if tag != TagASN1 {
		return fmt.Errorf("unexpected top level tag (%d)", tag)
	}
	if size < 2 {
		return fmt.Errorf("unexpected packet size (%d)", size)
	}
	// Read rest of the packet
	packetContent, err := ReadBytes(in, int(size))
	if err != nil {
		return
	}
	// Unmarshal SNMP version, keep in mind that 0 is SNMPv1, 1 is SNMPv2 and v2c.
	packet.Version, err = UnmarshalInteger(&packetContent)
	if err != nil {
		return
	}
	if packet.Version != ProtocolV2C {
		return fmt.Errorf("unexpected version number (%d)", packet.Version)
	}
	// Unmarshal SNMP community name
	packet.CommunityName, err = UnmarshalString(&packetContent)
	if err != nil {
		return
	}
	if packet.CommunityName == "" {
		return errors.New("missing community name")
	}
	// Pick up PDU
	if len(packetContent) < 2 {
		return errors.New("premature end of packet")
	}
	packet.PDU = packetContent[0]
	switch packet.PDU {
	case PDUGetRequest:
	case PDUGetNextRequest:
	default:
		return fmt.Errorf("unexpected PDU (%d)", packet.PDU)
	}
	// Skip PDU and size
	packetContent = packetContent[2:]
	// Unmarshal RequestID, ErrorStatus, ErrorIndex.
	packet.RequestID, err = UnmarshalInteger(&packetContent)
	if err != nil {
		return
	}
	packet.ErrorStatus, err = UnmarshalInteger(&packetContent)
	if err != nil {
		return
	}
	packet.ErrorIndex, err = UnmarshalInteger(&packetContent)
	if err != nil {
		return
	}
	// Further dissect the content
	switch packet.PDU {
	case PDUGetRequest:
		packet.Structure, err = ReadGetRequest(packetContent)
	case PDUGetNextRequest:
		packet.Structure, err = ReadGetNextRequest(packetContent)
	}
	return
}

// Encode encodes a response packet into a byte array.
func (packet *Packet) Encode() (ret []byte, err error) {
	ret = make([]byte, 0, 256)
	var packetContentSize byte
	// Calculation of packet size (index 1) and packet content size (index 3) is ongoing
	ret = append(ret, TagASN1, packetContentSize)

	// Encode SNMP version
	versionBytes, err := asn1.Marshal(packet.Version)
	if err != nil {
		return
	}
	packetContentSize += byte(len(versionBytes))
	ret = append(ret, versionBytes...)

	// Encode community Name
	communityNameBytes, err := asn1.Marshal([]byte(packet.CommunityName))
	if err != nil {
		return
	}
	packetContentSize += byte(len(communityNameBytes))
	ret = append(ret, communityNameBytes...)

	// PDU does not require ASN1 encoding
	ret = append(ret, packet.PDU)
	// But trailing PDU is yet another size calculation
	var sizeAfterPDU byte
	// Calculation of this size is ongoing
	sizeAfterPDUIndex := len(ret)
	ret = append(ret, sizeAfterPDU)

	// Encode request ID
	requestIDBytes, err := asn1.Marshal(packet.RequestID)
	if err != nil {
		return
	}
	packetContentSize += byte(len(requestIDBytes))
	sizeAfterPDU += byte(len(requestIDBytes))
	ret = append(ret, requestIDBytes...)

	// Encode NoError and ErrorIndex 0
	noErrorBytes, err := asn1.Marshal(0)
	if err != nil {
		return
	}
	packetContentSize += byte(len(noErrorBytes))
	sizeAfterPDU += byte(len(noErrorBytes))
	ret = append(ret, noErrorBytes...)
	errIndex0, err := asn1.Marshal(0)
	if err != nil {
		return
	}
	packetContentSize += byte(len(errIndex0))
	sizeAfterPDU += byte(len(errIndex0))
	ret = append(ret, errIndex0...)

	// Subsequent content is determined by PDU
	var subsequentContent []byte
	switch st := packet.Structure.(type) {
	case GetResponse:
		subsequentContent, err = st.Encode()
		if err != nil {
			return
		}
	case GetNextResponse:
		subsequentContent, err = st.Encode()
		if err != nil {
			return
		}
	default:
		err = errors.New("programming mistake - it will not encode a request")
		return
	}
	packetContentSize += byte(len(subsequentContent))
	sizeAfterPDU += byte(len(subsequentContent))
	ret = append(ret, subsequentContent...)

	ret[sizeAfterPDUIndex] = sizeAfterPDU
	ret[1] = packetContentSize + 2
	return
}

// GetNextRequest describes a request for exactly one subsequent OID during a walk operation.
type GetNextRequest struct {
	// BaseOID is the base OID to descend from.
	BaseOID asn1.ObjectIdentifier
}

// ReadGetNextRequest dissects the content of a GetNextRequest from input.
func ReadGetNextRequest(in []byte) (req GetNextRequest, err error) {
	if len(in) < 4 {
		return GetNextRequest{}, fmt.Errorf("unexpected content size for GetNextReuqest (%d)", len(in))
	}
	// Skip four bytes - ASN.1, SZ, ASN.1, SZ, and then read OID.
	_, err = asn1.Unmarshal(in[4:], &req.BaseOID)
	// Ignore a null value that might trail OID
	return
}

// GetRequest describes a request for a value corresponding to requested OID.
type GetRequest struct {
	// RequestedOID is the desired OID.
	RequestedOID asn1.ObjectIdentifier
}

// ReadGetRequest dissects the content of a GetNextRequest from input.
func ReadGetRequest(in []byte) (req GetRequest, err error) {
	if len(in) < 4 {
		return GetRequest{}, fmt.Errorf("unexpected content size for GetNextReuqest (%d)", len(in))
	}
	// Requested OID is right there, there are no prefix bytes leading to it.
	_, err = asn1.Unmarshal(in[4:], &req.RequestedOID)
	return
}

// GetNextRequest describes an answer toward GetNextRequest.
type GetNextResponse struct {
	// ResponseOIDs are the sensible subsequent OIDs.
	ResponseOIDs []asn1.ObjectIdentifier
}

// Encode encodes the response into a byte array as answer toward a GetNextRequest.
func (resp GetNextResponse) Encode() (ret []byte, err error) {
	ret = make([]byte, 4)
	lenItems := 0
	for _, oid := range resp.ResponseOIDs {
		respOIDBytes, err := asn1.Marshal(oid)
		if err != nil {
			return []byte{}, nil
		}
		lenItems += len(respOIDBytes)
		ret = append(ret, respOIDBytes...)
	}

	// ASN.1, size of items + 2
	ret[0] = 0x30
	ret[1] = byte(2 + lenItems)
	// ASN.1, size of items
	ret[2] = 0x30
	ret[3] = byte(lenItems)
	return
}

// GetRequest describes an answer toward GetRequest.
type GetResponse struct {
	// RequestedOID is the OID presented in corresponding request.
	RequestedOID asn1.ObjectIdentifier
	// Value is the primitive value corresponding to requested OID.
	Value interface{}
	// NoSuchInstance describes a response toward non-existing OID.
	NoSuchInstance bool
	// EndOfMIBView describes a response toward the OID immediately beyond the last one in hierarchy, hence the OID does not exist.
	EndOfMIBView bool
}

// Encode encodes the response into a byte array as answer toward a GetRequest.
func (resp GetResponse) Encode() (ret []byte, err error) {
	reqOIDBytes, err := asn1.Marshal(resp.RequestedOID)
	if err != nil {
		return []byte{}, nil
	}

	var primitive []byte
	if resp.NoSuchInstance {
		primitive = []byte{TagNoSuchInstance, 0x00}
	} else if resp.EndOfMIBView {
		primitive = []byte{TagEndOfMIBView, 0x00}
	} else {
		if primitive, err = asn1.Marshal(resp.Value); err != nil {
			return []byte{}, nil
		}
	}

	lenItems := len(reqOIDBytes) + len(primitive)
	ret = make([]byte, 4, lenItems)
	// ASN.1, size of items + 2
	ret[0] = 0x30
	ret[1] = byte(2 + lenItems)
	// ASN.1, size of items
	ret[2] = 0x30
	ret[3] = byte(lenItems)
	ret = append(ret, reqOIDBytes...)
	ret = append(ret, primitive...)
	return
}
