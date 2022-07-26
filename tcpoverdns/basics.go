package tcpoverdns

import (
	"bytes"
	"compress/flate"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"golang.org/x/net/dns/dnsmessage"
)

var base32EncodingNoPadding = base32.StdEncoding.WithPadding(base32.NoPadding)

// State is the transmission control stream's state.
type State int

const (
	StateEmpty       = State(0)
	StateSynReceived = State(1)
	StatePeerAck     = State(2)
	StateEstablished = State(3)
	StatePeerClosed  = State(4)
	StateClosed      = State(100)
)

// Flag is transmitted with each segment, it is the data type of an individual
// flag bit while also used to represent an entire collection of flags.
// Transmission control and its peer use flags to communicate transition between
// states.
type Flag uint16

const (
	FlagHandshakeSyn = Flag(1 << 0)
	FlagHandshakeAck = Flag(1 << 1)
	FlagAckOnly      = Flag(1 << 2)
	FlagKeepAlive    = Flag(1 << 3)
	// FlagReset asks the peer to close/terminate, as the local side also does.
	// The misleading name was inspired by TCP reset.
	FlagReset     = Flag(1 << 4)
	FlagMalformed = Flag(1 << 5)
)

func (flag Flag) Has(f Flag) bool {
	return flag&f != 0
}

func (flag Flag) String() string {
	var names []string
	if flag.Has(FlagHandshakeSyn) {
		names = append(names, "HandshakeSyn")
	}
	if flag.Has(FlagHandshakeAck) {
		names = append(names, "HandshakeAck")
	}
	if flag.Has(FlagAckOnly) {
		names = append(names, "AckOnly")
	}
	if flag.Has(FlagKeepAlive) {
		names = append(names, "KeepAlive")
	}
	if flag.Has(FlagReset) {
		names = append(names, "ResetTerminate")
	}
	if flag.Has(FlagMalformed) {
		names = append(names, "Malformed")
	}
	return strings.Join(names, "+")
}

const (
	// SegmentHeaderLen is the total length of a segment header.
	SegmentHeaderLen = 14
)

// Segment is a unit of data transported by TransmissionControl. A stream of
// longer data length is broken down into individual segments before they are
// transported.
type Segment struct {
	// Flags is a bitmap of individual control bits that help the stream to
	// transition between its states.
	Flags Flag
	// ID is the ID of the transmission control that constructed this segment.
	ID uint16
	// SeqNum is the sequence number of the first byte of data of the segment.
	SeqNum uint32
	// AckNum differs from the way it works in TCP. Over here it is the sequence
	// number of the latest byte arrived, whereas in TCP it is the next sequence
	// number expected from peer - oops!
	AckNum uint32
	Data   []byte
}

func (seg *Segment) Equals(other Segment) bool {
	return seg.Flags == other.Flags &&
		seg.ID == other.ID &&
		seg.SeqNum == other.SeqNum &&
		seg.AckNum == other.AckNum &&
		bytes.Equal(seg.Data, other.Data)
}

// Packet serialises the segment into bytes and returns them.
func (seg *Segment) Packet() (ret []byte) {
	ret = make([]byte, 2+2+4+4+2+len(seg.Data))
	binary.BigEndian.PutUint16(ret[0:2], seg.ID)
	binary.BigEndian.PutUint16(ret[2:4], uint16(seg.Flags))
	binary.BigEndian.PutUint32(ret[4:8], seg.SeqNum)
	binary.BigEndian.PutUint32(ret[8:12], seg.AckNum)
	binary.BigEndian.PutUint16(ret[12:14], uint16(len(seg.Data)))
	copy(ret[SegmentHeaderLen:], seg.Data)
	return
}

// DNSQuestion converts the binary representation of this segment into a DNS
// query question - "prefix.seg.seg.seg...domainName".
// The function does not check whether the segment is sufficiently small for
// the DNS protocol.
func (seg *Segment) DNSQuestion(prefix, domainName string) dnsmessage.Question {
	// Compress the binary representation of the segment.
	packet := seg.Packet()
	compressed := CompressBytes(packet)
	// Encode using base32.
	encoded := strings.ToLower(base32EncodingNoPadding.EncodeToString(compressed))
	// Split into 63 characters per label.
	// 63 is the maximum label size decided by the DNS protocol.
	labels := misc.SplitIntoSlice(encoded, 63, MaxSegmentDataLen*2)
	return dnsmessage.Question{
		Name:  dnsmessage.MustNewName(fmt.Sprintf(`%s.%s.%s`, prefix, strings.Join(labels, "."), domainName)),
		Type:  dnsmessage.TypeA,
		Class: dnsmessage.ClassINET,
	}
}

// DNSResource converts the binary representation of this segment into a DNS
// address resource. The function does not check whether the segment is
// sufficiently small for the DNS protocol.
func (seg *Segment) DNSResource() (ret []dnsmessage.AResource) {
	packet := seg.Packet()
	compressed := CompressBytes(packet)
	// Add the length prefix.
	lenPrefix := make([]byte, 2)
	binary.BigEndian.PutUint16(lenPrefix, uint16(len(compressed)))
	// Split into address resource records.
	compressed = append(lenPrefix, compressed...)
	for i := 0; i < len(compressed); i += 4 {
		end := i + 4
		if end > len(compressed) {
			end = len(compressed)
		}
		addr := [4]byte{}
		copy(addr[:], compressed[i:end])
		ret = append(ret, dnsmessage.AResource{A: addr})
	}
	return
}

// Stringer returns a human-readable representation of the segment for debug
// logging.
func (seg Segment) String() string {
	return fmt.Sprintf("[ID=%d Seq=%d Ack=%d Flags=%v Data=%s]", seg.ID, seg.SeqNum, seg.AckNum, seg.Flags, lalog.ByteArrayLogString(seg.Data))
}

// SegmentFromPacket decodes a segment from a byte array and returns the decoded
// segment.
func SegmentFromPacket(packet []byte) Segment {
	if len(packet) < SegmentHeaderLen {
		return Segment{Flags: FlagMalformed}
	}
	id := binary.BigEndian.Uint16(packet[0:2])
	flags := Flag(binary.BigEndian.Uint16(packet[2:4]))
	seq := binary.BigEndian.Uint32(packet[4:8])
	ack := binary.BigEndian.Uint32(packet[8:12])
	length := binary.BigEndian.Uint16(packet[12:14])
	if len(packet) < SegmentHeaderLen+int(length) {
		return Segment{Flags: FlagMalformed}
	}
	data := packet[SegmentHeaderLen : SegmentHeaderLen+length]
	seg := Segment{
		ID:     id,
		Flags:  flags,
		SeqNum: seq,
		AckNum: ack,
		Data:   data,
	}

	// The HandshakeSyn segment must have the initiator config.
	if seg.Flags == FlagHandshakeSyn {
		if len(seg.Data) < InitiatorConfigLen {
			return Segment{Flags: FlagMalformed}
		}
	}
	return seg
}

// SegmentFromDNSQuestion decodes a segment from a DNS question.
func SegmentFromDNSQuestion(numDomainNameLabels int, in dnsmessage.Question) Segment {
	labels := strings.Split(in.Name.String(), ".")
	if len(labels) < 1+1+numDomainNameLabels {
		return Segment{Flags: FlagMalformed}
	}
	// Recover base32 encoded binary data by concatenating the labels.
	labels = labels[1 : len(labels)-numDomainNameLabels]
	compressed, err := base32EncodingNoPadding.DecodeString(strings.ToUpper(strings.Join(labels, "")))
	if err != nil {
		return Segment{Flags: FlagMalformed}
	}
	// Decompress the binary packet.
	decompressed, err := DecompressBytes(compressed)
	if err != nil {
		return Segment{Flags: FlagMalformed}
	}
	return SegmentFromPacket(decompressed)
}

// SegmentFromDNSResources decodes a segment from DNS address resource records.
func SegmentFromDNSResources(in []dnsmessage.AResource) Segment {
	// Recover binary data from the resource records.
	data := make([]byte, 0)
	for _, rec := range in {
		data = append(data, rec.A[:]...)
	}
	if len(data) < 3 {
		return Segment{Flags: FlagMalformed}
	}
	// Decode the data length.
	segLen := binary.BigEndian.Uint16(data[:2])
	if len(data) < 2+int(segLen) {
		return Segment{Flags: FlagMalformed}
	}
	// Decompress the segment packet.
	decompressed, err := DecompressBytes(data[2 : 2+segLen])
	if err != nil {
		return Segment{Flags: FlagMalformed}
	}
	return SegmentFromPacket(decompressed)
}

// CompressBytes compresses the input byte array using a scheme with the best
// compress ratio.
func CompressBytes(original []byte) (compressed []byte) {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		panic(err)
	}
	_, _ = w.Write([]byte(original))
	_ = w.Close()
	return buf.Bytes()
}

// DecompressBytes recovers a byte array compressed by the CompressBytes
// function.
func DecompressBytes(compressed []byte) (original []byte, err error) {
	r := flate.NewReader(bytes.NewReader(compressed))
	original, err = io.ReadAll(r)
	return
}

const (
	// InitiatorConfigLen is the length of the serialised InitiatorConfig.
	InitiatorConfigLen = 19
)

// InitiatorConfig is a small piece of binary data inserted into the initiator's
// handshake segment during the handshake. The parameters help configure the
// responding transmission control.
type InitiatorConfig struct {
	// SetConfig instructs the responder to configure itself according to the
	// parameters specified here.
	SetConfig bool
	// MaxSegmentLenExclHeader is the maximum length of the data portion in an
	// outgoing segment, the length excludes the headers.
	MaxSegmentLenExclHeader int
	// ReadTimeout specifies a time limit for the Read function.
	ReadTimeout time.Duration
	// WriteTimeout specifies a time limit for the Write function.
	WriteTimeout time.Duration
}

// Bytes returns the binary data representation of the configuration parameters.
func (conf *InitiatorConfig) Bytes() []byte {
	ret := make([]byte, InitiatorConfigLen)
	if conf.SetConfig {
		ret[0] = 1
	}
	binary.BigEndian.PutUint16(ret[1:3], uint16(conf.MaxSegmentLenExclHeader))
	binary.BigEndian.PutUint64(ret[3:11], uint64(conf.ReadTimeout))
	binary.BigEndian.PutUint64(ret[11:19], uint64(conf.WriteTimeout))
	return ret
}

// Config copies the configuration parameters into the transmission control.
func (conf *InitiatorConfig) Config(tc *TransmissionControl) {
	if conf.SetConfig {
		tc.MaxSegmentLenExclHeader = conf.MaxSegmentLenExclHeader
		tc.ReadTimeout = conf.ReadTimeout
		tc.WriteTimeout = conf.WriteTimeout
	}
}

// DeserialiseInitiatorConfig decodes configuration parameters from the input
// byte array.
func DeserialiseInitiatorConfig(in []byte) *InitiatorConfig {
	ret := new(InitiatorConfig)
	ret.SetConfig = in[0] == 1
	ret.MaxSegmentLenExclHeader = int(binary.BigEndian.Uint16(in[1:3]))
	ret.ReadTimeout = time.Duration(binary.BigEndian.Uint64(in[3:11]))
	ret.WriteTimeout = time.Duration(binary.BigEndian.Uint64(in[11:19]))
	return ret
}
