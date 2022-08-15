package tcpoverdns

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
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
	// Split into labels.
	// 63 is the maximum label length decided by the DNS protocol.
	// But many recursive resolvers don't like long labels, so be conservative.
	labels := misc.SplitIntoSlice(encoded, 60, MaxSegmentDataLen)
	return dnsmessage.Question{
		Name:  dnsmessage.MustNewName(fmt.Sprintf(`%s.%s.%s`, prefix, strings.Join(labels, "."), domainName)),
		Type:  dnsmessage.TypeA,
		Class: dnsmessage.ClassINET,
	}
}

// DNSQuestion converts the binary representation of this segment into a DNS
// name query - "prefix.seg.seg.seg...domainName".
// The function does not check whether the segment is sufficiently small for
// the DNS protocol.
func (seg *Segment) DNSNameQuery(prefix, domainName string) string {
	// Compress the binary representation of the segment.
	packet := seg.Packet()
	compressed := CompressBytes(packet)
	// Encode using base32.
	encoded := strings.ToLower(base32EncodingNoPadding.EncodeToString(compressed))
	// Split into labels.
	// 63 is the maximum label length decided by the DNS protocol.
	// But many recursive resolvers don't like long labels, so be conservative.
	labels := misc.SplitIntoSlice(encoded, 60, MaxSegmentDataLen)
	return fmt.Sprintf(`%s.%s.%s`, prefix, strings.Join(labels, "."), domainName)
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
	for i := 0; i < len(compressed); i += 3 {
		addr := [4]byte{}
		addr[0] = compressed[i]
		if i+1 < len(compressed) {
			addr[1] = compressed[i+1]
		}
		addr[2] = byte(i) // the index byte
		if i+2 < len(compressed) {
			addr[3] = compressed[i+2]
		}
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

// SegmentFromDNSQuery decodes a segment from a DNS query.
func SegmentFromDNSQuery(numDomainNameLabels int, query string) Segment {
	if len(query) < 3 {
		return Segment{Flags: FlagMalformed}
	}
	if query[len(query)-1] == '.' {
		query = query[:len(query)-1]
	}
	labels := strings.Split(query, ".")
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

// SegmentFromDNSResources decodes a segment from IP addresses from a DNS query
// response.
func SegmentFromIPs(in []net.IP) Segment {
	// Put the IP entries in the original order.
	ordered := make([]net.IP, len(in))
	copy(ordered, in)
	sort.Slice(ordered, func(a, b int) bool {
		return ordered[a][2] < ordered[b][2]
	})
	// Recover binary data from the addresses.
	data := make([]byte, 0)
	for _, addr := range ordered {
		// addr[2] is the index.
		data = append(data, addr[0], addr[1], addr[3])
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
	InitiatorConfigLen = 8
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
	// IOTimeoutSec is the time limit (in seconds) for both read and write
	// functions.
	IOTimeoutSec int
	// KeepAliveIntervalSec is the keep alive interval in seconds.
	KeepAliveIntervalSec int
	// Debug asks the transmission control to turn on debug logging.
	Debug bool
}

// Bytes returns the binary data representation of the configuration parameters.
func (conf *InitiatorConfig) Bytes() []byte {
	ret := make([]byte, InitiatorConfigLen)
	if conf.SetConfig {
		ret[0] = 1
	}
	binary.BigEndian.PutUint16(ret[1:3], uint16(conf.MaxSegmentLenExclHeader))
	binary.BigEndian.PutUint16(ret[3:5], uint16(conf.IOTimeoutSec))
	binary.BigEndian.PutUint16(ret[5:7], uint16(conf.KeepAliveIntervalSec))
	if conf.Debug {
		ret[7] = 1
	}
	return ret
}

// Config copies the configuration parameters into the transmission control.
func (conf *InitiatorConfig) Config(tc *TransmissionControl) {
	if conf.SetConfig {
		if conf.MaxSegmentLenExclHeader > 0 {
			tc.MaxSegmentLenExclHeader = conf.MaxSegmentLenExclHeader
			tc.MaxSlidingWindow = uint32(8 * conf.MaxSegmentLenExclHeader)
		}
		tc.ReadTimeout = time.Duration(conf.IOTimeoutSec) * time.Second
		tc.WriteTimeout = tc.ReadTimeout
		tc.KeepAliveInterval = time.Duration(conf.KeepAliveIntervalSec) * time.Second
		tc.Debug = conf.Debug || tc.Debug
	}
}

// DeserialiseInitiatorConfig decodes configuration parameters from the input
// byte array.
func DeserialiseInitiatorConfig(in []byte) *InitiatorConfig {
	ret := new(InitiatorConfig)
	ret.SetConfig = in[0] == 1
	ret.MaxSegmentLenExclHeader = int(binary.BigEndian.Uint16(in[1:3]))
	ret.IOTimeoutSec = int(binary.BigEndian.Uint16(in[3:5]))
	ret.KeepAliveIntervalSec = int(binary.BigEndian.Uint16(in[5:7]))
	ret.Debug = in[7] == 1
	return ret
}

func ReadSegment(ctx context.Context, in io.Reader) Segment {
	segHeader := make([]byte, SegmentHeaderLen)
	n, err := in.Read(segHeader)
	if err != nil || n != SegmentHeaderLen {
		return Segment{Flags: FlagMalformed}
	}
	segDataLen := int(binary.BigEndian.Uint16(segHeader[SegmentHeaderLen-2 : SegmentHeaderLen]))
	segData := make([]byte, segDataLen)
	n, err = in.Read(segData)
	if err != nil || n != segDataLen {
		return Segment{Flags: FlagMalformed}
	}
	return SegmentFromPacket(append(segHeader, segData...))
}

func ReadSegmentHeaderData(t testingstub.T, ctx context.Context, in io.Reader) Segment {
	segHeader := make([]byte, SegmentHeaderLen)
	n, err := in.Read(segHeader)
	if err != nil || n != SegmentHeaderLen {
		t.Fatalf("failed to read segment header: %v %v", n, err)
		return Segment{}
	}

	segDataLen := int(binary.BigEndian.Uint16(segHeader[SegmentHeaderLen-2 : SegmentHeaderLen]))
	segData := make([]byte, segDataLen)
	n, err = in.Read(segData)
	if err != nil || n != segDataLen {
		t.Fatalf("failed to read segment data: %v %v", segDataLen, err)
		return Segment{}
	}

	return SegmentFromPacket(append(segHeader, segData...))
}

func CheckTC(t testingstub.T, tc *TransmissionControl, timeoutSec int, wantState State, wantInputSeq, wantInputAck, wantOutputSeq int, wantInputBuf, wantOutputBuf []byte) {
	t.Helper()
	for i := 0; i < timeoutSec*10; i++ {
		tc.mutex.Lock()
		instant := *tc
		tc.mutex.Unlock()
		if !reflect.DeepEqual(instant.state, wantState) {
			goto retry
		} else if int(instant.inputSeq) != wantInputSeq {
			goto retry
		} else if int(instant.inputAck) != wantInputAck {
			goto retry
		} else if int(instant.outputSeq) != wantOutputSeq {
			goto retry
		} else if (instant.inputBuf != nil && wantInputBuf != nil) && !reflect.DeepEqual(instant.inputBuf, wantInputBuf) {
			goto retry
		} else if (instant.outputBuf != nil && wantOutputBuf != nil) && !reflect.DeepEqual(instant.outputBuf, wantOutputBuf) {
			goto retry
		} else {
			return
		}
	retry:
		time.Sleep(100 * time.Millisecond)
	}
	tc.DumpState()
	t.Fatalf("want state: %d, input seq: %d, input ack: %d, output seq: %d, input buf: %v, output buf: %v",
		wantState, wantInputSeq, wantInputAck, wantOutputSeq, wantInputBuf, wantOutputBuf)
}

func CheckTCError(t testingstub.T, tc *TransmissionControl, timeoutSec int, wantOngoingTransmission, wantInputTransportErrors, wantOutputTransportErrors int) {
	t.Helper()
	for i := 0; i < timeoutSec*10; i++ {
		tc.mutex.Lock()
		instant := *tc
		tc.mutex.Unlock()
		if instant.ongoingRetransmissions != wantOngoingTransmission {
			goto retry
		} else if instant.inputTransportErrors != wantInputTransportErrors {
			goto retry
		} else if instant.outputTransportErrors != wantOutputTransportErrors {
			goto retry
		} else {
			return
		}
	retry:
		time.Sleep(100 * time.Millisecond)
	}
	tc.DumpState()
	t.Fatalf("want ongiong retrans: %d, input transport errs: %d, output transport errs: %d",
		wantOngoingTransmission, wantInputTransportErrors, wantOutputTransportErrors)
}
