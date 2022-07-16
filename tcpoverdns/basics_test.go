package tcpoverdns

import (
	"reflect"
	"testing"
)

func TestSegment_Packet(t *testing.T) {
	want := Segment{
		ID:     12345,
		Flags:  FlagHandshakeAck & FlagHandshakeSyn,
		SeqNum: 23456,
		AckNum: 34567,
		Data:   []byte{1, 2, 3, 4},
	}

	packet := want.Packet()
	got := SegmentFromPacket(packet)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recovered: %+#v original: %+#v", got, want)
	}
}

func TestSegmentFromPacket(t *testing.T) {
	want := Segment{Flags: FlagMalformed}
	segWithData := Segment{Data: []byte{1, 2}}
	segWithMalformedLen := segWithData.Packet()
	for _, seg := range [][]byte{nil, {1}, segWithMalformedLen[:SegmentHeaderLen+1]} {
		if got := SegmentFromPacket(seg); !reflect.DeepEqual(got, want) {
			t.Fatalf("got: %+#v, want: %+#v", got, want)
		}
	}
}

func TestFlags(t *testing.T) {
	allFlags := FlagHandshakeSyn | FlagHandshakeAck | FlagAckOnly | FlagKeepAlive | FlagReset | FlagMalformed
	for _, flag := range []Flag{FlagHandshakeSyn, FlagHandshakeAck, FlagAckOnly, FlagKeepAlive, FlagReset, FlagMalformed} {
		if !allFlags.Has(flag) {
			t.Fatalf("missing %d", flag)
		}
	}
	if allFlags.Has(1 << 6) {
		t.Fatalf("should not have had flag %d", 1<<4)
	}
}

func TestSegment_Equals(t *testing.T) {
	original := Segment{
		ID:     12345,
		Flags:  FlagHandshakeAck & FlagHandshakeSyn,
		SeqNum: 23456,
		AckNum: 34567,
		Data:   []byte{1, 2, 3, 4},
	}
	if !original.Equals(original) {
		t.Errorf("should have been equal")
	}

	tests := []struct {
		a, b Segment
	}{
		{a: Segment{ID: 1}, b: Segment{ID: 2}},
		{a: Segment{Flags: 1}, b: Segment{Flags: 2}},
		{a: Segment{SeqNum: 1}, b: Segment{SeqNum: 2}},
		{a: Segment{AckNum: 1}, b: Segment{AckNum: 2}},
		{a: Segment{Data: []byte{0}}, b: Segment{Data: []byte{1}}},
	}
	for _, test := range tests {
		if test.a.Equals(test.b) {
			t.Errorf("should not have been equal: %+v, %+v", test.a, test.b)
		}
	}
}
