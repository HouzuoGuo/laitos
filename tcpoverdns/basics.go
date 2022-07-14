package tcpoverdns

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/HouzuoGuo/laitos/lalog"
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
	return Segment{
		ID:     id,
		Flags:  flags,
		SeqNum: seq,
		AckNum: ack,
		Data:   data,
	}
}
