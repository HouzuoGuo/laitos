package tcpoverdns

import (
	"testing"

	"github.com/HouzuoGuo/laitos/lalog"
)

func TestSegmentBuffer(t *testing.T) {
	buf := NewSegmentBuffer(*lalog.DefaultLogger, true, 10)
	buf.Absorb(Segment{Flags: FlagKeepAlive})
	// The keep-alive segment should come with random data.
	if len(buf.backlog) != 1 {
		t.Fatalf("%+v", buf.backlog)
	}
	if seg, exists := buf.Latest(); !exists || seg.Reserved == 0 || seg.Flags != FlagKeepAlive {
		t.Fatalf("%+v", buf.backlog)
	}
	// Add an ack, it should replace the keep-alive.
	buf.Absorb(Segment{Flags: FlagAckOnly, AckNum: 12})
	if len(buf.backlog) != 1 {
		t.Fatalf("%+v", buf.backlog)
	}
	if seg, exists := buf.Latest(); !exists || !seg.Equals(Segment{Flags: FlagAckOnly, AckNum: 12}) {
		t.Fatalf("%+v", buf.backlog)
	}
	// Add a data segment, it should replace the ack.
	buf.Absorb(Segment{AckNum: 12, Data: []byte{0}})
	if len(buf.backlog) != 1 {
		t.Fatalf("%+v", buf.backlog)
	}
	if seg, exists := buf.Latest(); !exists || !seg.Equals(Segment{AckNum: 12, Data: []byte{0}}) {
		t.Fatalf("%+v", buf.backlog)
	}
	// Add a duplicated data segment.
	buf.Absorb(Segment{AckNum: 12, Data: []byte{0}})
	if len(buf.backlog) != 1 {
		t.Fatalf("%+v", buf.backlog)
	}
	if seg, exists := buf.Latest(); !exists || !seg.Equals(Segment{AckNum: 12, Data: []byte{0}}) {
		t.Fatalf("%+v", buf.backlog)
	}
	// Add a non-duplicate data segment and pop both segments.
	// Both segments have the same seq number, so they won't be merged.
	buf.Absorb(Segment{AckNum: 24, Data: []byte{1}})
	popped, exists := buf.Pop()
	if len(buf.backlog) != 1 || !exists || !popped.Equals(Segment{AckNum: 24, Data: []byte{0}}) {
		t.Fatalf("%+v, %+v, %+v", popped, exists, buf.backlog)
	}
	popped, exists = buf.Pop()
	if len(buf.backlog) != 0 || !exists || !popped.Equals(Segment{AckNum: 24, Data: []byte{1}}) {
		t.Fatalf("%+v, %+v, %+v", popped, exists, buf.backlog)
	}
	popped, exists = buf.Pop()
	if len(buf.backlog) != 0 || exists {
		t.Fatalf("%+v, %+v, %+v", popped, exists, buf.backlog)
	}
}

func TestSegmentBuffer_MergeSeg(t *testing.T) {
	buf := NewSegmentBuffer(*lalog.DefaultLogger, true, 10)
	// These two segments will merge into a single segment.
	buf.Absorb(Segment{SeqNum: 0, AckNum: 1, Data: []byte{0, 1, 2}})
	buf.Absorb(Segment{SeqNum: 3, AckNum: 2, Data: []byte{3, 4, 5}})
	if len(buf.backlog) != 1 {
		t.Fatalf("%+v", buf.backlog)
	}
	if seg, exists := buf.First(); !exists || !seg.Equals(Segment{SeqNum: 0, AckNum: 2, Data: []byte{0, 1, 2, 3, 4, 5}}) {
		t.Fatalf("%+v", buf.backlog)
	}
	if seg, exists := buf.Latest(); !exists || !seg.Equals(Segment{SeqNum: 0, AckNum: 2, Data: []byte{0, 1, 2, 3, 4, 5}}) {
		t.Fatalf("%+v", buf.backlog)
	}
	// This next segment is too large to merge into a single segment.
	buf.Absorb(Segment{SeqNum: 6, AckNum: 2, Data: []byte{3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3}})
	if len(buf.backlog) != 2 {
		t.Fatalf("%+v", buf.backlog)
	}
	if seg, exists := buf.Latest(); !exists || !seg.Equals(Segment{SeqNum: 6, AckNum: 2, Data: []byte{3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3}}) {
		t.Fatalf("%+v", buf.backlog)
	}
	// Followed by a keep-alive.
	buf.Absorb(Segment{SeqNum: 6, AckNum: 2, Data: []byte{99, 99, 99}, Flags: FlagKeepAlive})
	if len(buf.backlog) != 3 {
		t.Fatalf("%+v", buf.backlog)
	}
	latest, exists := buf.Latest()
	if !exists || latest.SeqNum != 6 || latest.AckNum != 2 || latest.Reserved == 0 || latest.Flags != FlagKeepAlive {
		t.Fatalf("%v, %+v", exists, latest)
	}
	// Simulate two retransmissions.
	buf.Absorb(Segment{SeqNum: 3, AckNum: 2, Data: []byte{3, 4, 5}})
	if len(buf.backlog) != 1 {
		t.Fatalf("%+v", buf.backlog)
	}
	if seg, exists := buf.Latest(); !exists || !seg.Equals(Segment{SeqNum: 3, AckNum: 2, Data: []byte{3, 4, 5}}) {
		t.Fatalf("%+v", buf.backlog)
	}
	buf.Absorb(Segment{SeqNum: 1, AckNum: 2, Data: []byte{0, 1}})
	if len(buf.backlog) != 1 {
		t.Fatalf("%+v", buf.backlog)
	}
	if seg, exists := buf.Latest(); !exists || !seg.Equals(Segment{SeqNum: 1, AckNum: 2, Data: []byte{0, 1}}) {
		t.Fatalf("%+v", buf.backlog)
	}
}
