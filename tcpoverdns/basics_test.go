package tcpoverdns

import (
	"reflect"
	"testing"
)

func TestSegment_Packet(t *testing.T) {
	want := Segment{
		Flags:  FlagAck & FlagSyn,
		SeqNum: 1,
		AckNum: 2,
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
	allFlags := FlagSyn | FlagAck | FlagEnd
	for _, flag := range []Flag{FlagSyn, FlagAck, FlagEnd} {
		if !allFlags.Has(flag) {
			t.Fatalf("missing %d", flag)
		}
	}
	if allFlags.Has(1 << 4) {
		t.Fatalf("should not have had flag %d", 1<<4)
	}
}
