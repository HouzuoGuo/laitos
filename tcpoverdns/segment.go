package tcpoverdns

import "encoding/binary"

const (
	// SegmentHeaderLen is the total length of a segment header.
	SegmentHeaderLen = 10
)

// Segment is a unit of data transported by TransmissionControl. A stream of
// longer data length is broken down into individual segments before they are
// transported.
type Segment struct {
	SeqNum int
	AckNum int
	Data   []byte
}

func (seg *Segment) Packet() (ret []byte) {
	ret = make([]byte, 4+4+2+len(seg.Data))
	binary.BigEndian.PutUint32(ret[0:4], uint32(seg.SeqNum))
	binary.BigEndian.PutUint32(ret[4:8], uint32(seg.AckNum))
	binary.BigEndian.PutUint16(ret[8:10], uint16(len(seg.Data)))
	copy(ret[10:], seg.Data)
	return
}

func SegmentFromPacket(packet []byte) Segment {
	// FIXME: ensure the packet length >= 8
	seq := binary.BigEndian.Uint32(packet[0:4])
	ack := binary.BigEndian.Uint32(packet[4:8])
	// FIXME: ensure length is sane and not out of bound
	length := binary.BigEndian.Uint16(packet[8:10])
	data := packet[10 : 10+length]
	return Segment{
		SeqNum: int(seq),
		AckNum: int(ack),
		Data:   data,
	}
}
