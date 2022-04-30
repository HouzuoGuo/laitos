package dnspe

import (
	"reflect"
	"testing"
)

func TestSegment(t *testing.T) {
	seg := Segment{
		SeqNum: 1,
		AckNum: 2,
		Data:   []byte{1, 2, 3, 4},
	}

	packet := seg.Packet()
	recovered := SegmentFromPacket(packet)
	if !reflect.DeepEqual(recovered, seg) {
		t.Fatalf("recovered: %+#v original: %+#v", recovered, seg)
	}
}
