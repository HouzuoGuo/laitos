package tcpoverdns

import "encoding/binary"

// State is the transmission control stream's state.
type State int

const (
	StateEmpty       = State(0)
	StateSynReceived = State(2)
	StatePeerAck     = State(4)
	StateEstablished = State(5)
	StatePeerClosed  = State(5)
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
	FlagReset        = Flag(1 << 4)
	FlagMalformed    = Flag(1 << 5)
)

func (flag Flag) Has(f Flag) bool {
	return flag&f != 0
}

const (
	// SegmentHeaderLen is the total length of a segment header.
	SegmentHeaderLen = 12
)

// Segment is a unit of data transported by TransmissionControl. A stream of
// longer data length is broken down into individual segments before they are
// transported.
type Segment struct {
	// Flags is a bitmap of individual control bits that help the stream to
	// transition between its states.
	Flags Flag
	// ID is a random number
	ID uint16
	// SeqNum is the sequence number of the first byte of data of the segment.
	SeqNum uint32
	// AckNum differs from the way it works in TCP. Over here it is the sequence
	// number of the latest byte arrived, whereas in TCP it is the next sequence
	// number expected from peer - oops!
	AckNum uint32
	Data   []byte
}

func (seg *Segment) Packet() (ret []byte) {
	ret = make([]byte, 2+4+4+2+len(seg.Data))
	binary.BigEndian.PutUint16(ret[0:2], uint16(seg.Flags))
	binary.BigEndian.PutUint32(ret[2:6], seg.SeqNum)
	binary.BigEndian.PutUint32(ret[6:10], seg.AckNum)
	binary.BigEndian.PutUint16(ret[10:12], uint16(len(seg.Data)))
	copy(ret[SegmentHeaderLen:], seg.Data)
	return
}

func SegmentFromPacket(packet []byte) Segment {
	if len(packet) < SegmentHeaderLen {
		return Segment{Flags: FlagMalformed}
	}
	flags := Flag(binary.BigEndian.Uint16(packet[0:2]))
	seq := binary.BigEndian.Uint32(packet[2:6])
	ack := binary.BigEndian.Uint32(packet[6:10])
	length := binary.BigEndian.Uint16(packet[10:12])
	if len(packet) < SegmentHeaderLen+int(length) {
		return Segment{Flags: FlagMalformed}
	}
	data := packet[SegmentHeaderLen : SegmentHeaderLen+length]
	return Segment{
		Flags:  flags,
		SeqNum: seq,
		AckNum: ack,
		Data:   data,
	}
}
