package tcpoverdns

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"reflect"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
)

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
	SegmentHeaderLen = 16
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
	// Reserved is a two-byte integer. It is currently used to inject a small
	// amount of randomness into the segment.
	Reserved uint16
	Data     []byte
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
	ret = make([]byte, 2+2+4+4+2+2+len(seg.Data))
	binary.BigEndian.PutUint16(ret[0:2], seg.ID)
	binary.BigEndian.PutUint16(ret[2:4], uint16(seg.Flags))
	binary.BigEndian.PutUint32(ret[4:8], seg.SeqNum)
	binary.BigEndian.PutUint32(ret[8:12], seg.AckNum)
	binary.BigEndian.PutUint16(ret[12:14], seg.Reserved)
	binary.BigEndian.PutUint16(ret[14:16], uint16(len(seg.Data)))
	copy(ret[SegmentHeaderLen:], seg.Data)
	return
}

// CompressAndEncode compresses and encodes the segment into a string.
func (seg *Segment) CompressAndEncode() string {
	packet := seg.Packet()
	compressed := CompressBytes(packet)
	return ToBase62Mod(compressed)
}

// DNSName converts the binary representation of this segment into a DNS name -
// "prefix.seg.seg.seg...domainName". The return string does not have a suffix
// period.
// The function does not check whether the segment is sufficiently small for
// the DNS protocol.
func (seg *Segment) DNSName(prefix, domainName string) string {
	if len(prefix) == 0 || len(domainName) == 0 {
		return ""
	}
	if domainName[len(domainName)-1] != '.' {
		domainName += "."
	}
	encoded := seg.CompressAndEncode()
	// Split into labels.
	// 63 is the maximum label length decided by the DNS protocol.
	// But many recursive resolvers don't like long labels, so be conservative.
	labels := misc.SplitIntoSlice(encoded, 60, MaxSegmentDataLen)
	return fmt.Sprintf(`%s.%s.%s`, prefix, strings.Join(labels, "."), domainName)
}

// DNSText converts the binary representation of this segment into DNS text
// entries.
// The function does not restrict the maximum size of the text entries.
func (seg *Segment) DNSText() []string {
	encoded := seg.CompressAndEncode()
	return misc.SplitIntoSlice(encoded, 253, MaxSegmentDataLen)
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
		return Segment{Flags: FlagMalformed, Data: []byte("packet is shorter than header len")}
	}
	id := binary.BigEndian.Uint16(packet[0:2])
	flags := Flag(binary.BigEndian.Uint16(packet[2:4]))
	seq := binary.BigEndian.Uint32(packet[4:8])
	ack := binary.BigEndian.Uint32(packet[8:12])
	reserved := binary.BigEndian.Uint16(packet[12:14])
	length := binary.BigEndian.Uint16(packet[14:16])
	if len(packet) < SegmentHeaderLen+int(length) {
		return Segment{Flags: FlagMalformed, Data: []byte("data is shorter than advertised len")}
	}
	data := packet[SegmentHeaderLen : SegmentHeaderLen+length]
	seg := Segment{
		ID:       id,
		Flags:    flags,
		SeqNum:   seq,
		AckNum:   ack,
		Reserved: reserved,
		Data:     data,
	}

	// The HandshakeSyn segment must have the initiator config.
	if seg.Flags == FlagHandshakeSyn {
		if len(seg.Data) < InitiatorConfigLen {
			return Segment{Flags: FlagMalformed, Data: []byte("missing initiator config")}
		}
	}
	return seg
}

// SegmentFromDNSName decodes a segment from a DNS name, for example, the name
// of a query, or a CNAME from a response.
func SegmentFromDNSText(entries []string) Segment {
	if len(entries) == 0 {
		return Segment{Flags: FlagMalformed, Data: []byte("no text entries")}
	}
	compressed, err := ParseBase62Mod(strings.Join(entries, ""))
	if err != nil {
		return Segment{Flags: FlagMalformed, Data: []byte("failed to parse base62 data")}
	}
	// Decompress the binary packet.
	decompressed, err := DecompressBytes(compressed)
	if err != nil {
		return Segment{Flags: FlagMalformed, Data: []byte("failed to decompress data")}
	}
	return SegmentFromPacket(decompressed)
}

// SegmentFromDNSName decodes a segment from a DNS name, for example, the name
// of a query, or a CNAME from a response.
func SegmentFromDNSName(numDomainNameLabels int, query string) Segment {
	if len(query) < 3 {
		return Segment{Flags: FlagMalformed, Data: []byte("query is too short")}
	}
	// Remove trailing full-stop.
	if query[len(query)-1] == '.' {
		query = query[:len(query)-1]
	}
	labels := strings.Split(query, ".")
	// "prefix.data-data-data.mydomain.com"
	if len(labels) < 1+1+numDomainNameLabels {
		return Segment{Flags: FlagMalformed, Data: []byte("too few name labels")}
	}
	// Recover base32 encoded binary data by concatenating the labels.
	// The first label is a prefix only and does not carry binary data.
	labels = labels[1 : len(labels)-numDomainNameLabels]
	compressed, err := ParseBase62Mod(strings.Join(labels, ""))
	if err != nil {
		return Segment{Flags: FlagMalformed, Data: []byte("failed to parse base62 data")}
	}
	// Decompress the binary packet.
	decompressed, err := DecompressBytes(compressed)
	if err != nil {
		return Segment{Flags: FlagMalformed, Data: []byte("failed to decompress data")}
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
	original, err = ioutil.ReadAll(r)
	return
}

const (
	// InitiatorConfigLen is the length of the serialised InitiatorConfig.
	InitiatorConfigLen = 28
)

// TimingConfig has the timing characteristics of a transmission control.
type TimingConfig struct {
	// SlidingWindowWaitDuration is a short duration to wait the peer's
	// acknowledgement before this transmission control sends more data to the
	// output transport, including during consecutive retransmissions.
	// In practice, the duration should be higher than the initial
	// KeepAliveInterval.
	SlidingWindowWaitDuration time.Duration
	// RetransmissionInterval is a short duration to wait before re-transmitting
	// the unacknowledged outbound segments (if any). Subsequent retransmissions
	// occur at the interval of SlidingWindowWaitDuration.
	RetransmissionInterval time.Duration
	// AckDelay is a short delay between receiving the latest segment and
	// sending an outbound acknowledgement-only segment.
	// It should shorter than the retransmission interval by a magnitude.
	AckDelay time.Duration
	// KeepAliveInterval is a short duration to wait before transmitting an
	// outbound ack segment in the absence of outbound data.
	// This internal must be longer than the peer's retransmission interval.
	KeepAliveInterval time.Duration
	// ReadTimeout specifies a time limit for the Read function.
	ReadTimeout time.Duration
	// WriteTimeout specifies a time limit for the Write function.
	WriteTimeout time.Duration
}

// HalfInterval divides all interval timing attributes by half.
func (conf *TimingConfig) HalfInterval() {
	conf.SlidingWindowWaitDuration /= 2
	conf.AckDelay /= 2
	conf.KeepAliveInterval /= 2
}

// DoubleInterval doubles all interval timing attributes.
func (conf *TimingConfig) DoubleInterval() {
	conf.SlidingWindowWaitDuration *= 2
	conf.AckDelay *= 2
	conf.KeepAliveInterval *= 2
}

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
	// Debug enables verbose logging for IO activities.
	Debug bool
	// Timing configures the transmission control's timing
	// characteristics.
	Timing TimingConfig
}

// Bytes returns the binary data representation of the configuration parameters.
func (conf *InitiatorConfig) Bytes() []byte {
	ret := make([]byte, InitiatorConfigLen)
	if conf.SetConfig {
		ret[0] = 1
	}
	if conf.Debug {
		ret[1] = 1
	}
	binary.BigEndian.PutUint16(ret[2:4], uint16(conf.MaxSegmentLenExclHeader))
	// For timing configuration, round to the nearest milliseconds.
	binary.BigEndian.PutUint32(ret[4:8], uint32(conf.Timing.SlidingWindowWaitDuration/time.Millisecond))
	binary.BigEndian.PutUint32(ret[8:12], uint32(conf.Timing.RetransmissionInterval/time.Millisecond))
	binary.BigEndian.PutUint32(ret[12:16], uint32(conf.Timing.AckDelay/time.Millisecond))
	binary.BigEndian.PutUint32(ret[16:20], uint32(conf.Timing.ReadTimeout/time.Millisecond))
	binary.BigEndian.PutUint32(ret[20:24], uint32(conf.Timing.WriteTimeout/time.Millisecond))
	binary.BigEndian.PutUint32(ret[24:28], uint32(conf.Timing.KeepAliveInterval/time.Millisecond))
	return ret
}

// Config copies the configuration parameters into the transmission control.
func (conf *InitiatorConfig) Config(tc *TransmissionControl) {
	if conf.SetConfig {
		if conf.MaxSegmentLenExclHeader > 0 {
			tc.MaxSegmentLenExclHeader = conf.MaxSegmentLenExclHeader
			tc.MaxSlidingWindow = uint32(64 * conf.MaxSegmentLenExclHeader)
		}
		tc.Debug = conf.Debug || tc.Debug
		if conf.Timing.ReadTimeout > 0 {
			tc.InitialTiming = conf.Timing
			tc.LiveTiming = conf.Timing
		}
	}
}

// DeserialiseInitiatorConfig decodes configuration parameters from the input
// byte array.
func DeserialiseInitiatorConfig(in []byte) *InitiatorConfig {
	ret := new(InitiatorConfig)
	ret.SetConfig = in[0] == 1
	ret.Debug = in[1] == 1
	ret.MaxSegmentLenExclHeader = int(binary.BigEndian.Uint16(in[2:4]))
	// All timing configuration are in milliseconds.
	ret.Timing.SlidingWindowWaitDuration = time.Duration(binary.BigEndian.Uint32(in[4:8])) * time.Millisecond
	ret.Timing.RetransmissionInterval = time.Duration(binary.BigEndian.Uint32(in[8:12])) * time.Millisecond
	ret.Timing.AckDelay = time.Duration(binary.BigEndian.Uint32(in[12:16])) * time.Millisecond
	ret.Timing.ReadTimeout = time.Duration(binary.BigEndian.Uint32(in[16:20])) * time.Millisecond
	ret.Timing.WriteTimeout = time.Duration(binary.BigEndian.Uint32(in[20:24])) * time.Millisecond
	ret.Timing.KeepAliveInterval = time.Duration(binary.BigEndian.Uint32(in[24:28])) * time.Millisecond
	return ret
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

// ToBase62Mod encodes [1, input...] in a Base62-encoded string.
func ToBase62Mod(content []byte) string {
	withPrefix := make([]byte, len(content)+1)
	withPrefix[0] = 1
	copy(withPrefix[1:], content)
	var i big.Int
	i.SetBytes(withPrefix)
	return i.Text(62)
}

// ParseBase62Mod recovers the original content from a Base62Mod-encoded string.
func ParseBase62Mod(s string) ([]byte, error) {
	var i big.Int
	_, ok := i.SetString(s, 62)
	if !ok {
		return nil, fmt.Errorf("failed to parse base62 encoded string: %q", s)
	}
	recovered := i.Bytes()
	if len(recovered) == 0 {
		return nil, errors.New("the input is missing the Base62Mod prefix byte")
	}
	return recovered[1:], nil
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
