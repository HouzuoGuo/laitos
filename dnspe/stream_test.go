package dnspe

import (
	"context"
	"net"
	"reflect"
	"testing"
)

func TestTransmissionControl_Segments(t *testing.T) {
	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := TransmissionControl{
		MaxInTransitLen: 10,
		MaxSegmentLen:   5,
		InputTransport:  inTransport,
		OutputTransport: outTransport,
	}
	tc.Start(context.Background())
	// Test write.
	n, err := tc.Write([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	if n != 10 || err != nil {
		t.Fatalf("n: %+#v, err: %+#v", n, err)
	}
	buf := make([]byte, 10)
	// Read the first segment.
	n, err = testIn.Read(buf)
	if n != 5 || err != nil {
		t.Fatalf("n: %+#v, err: %+#v", n, err)
	}
	if !reflect.DeepEqual(buf[:n], []byte{0, 1, 2, 3, 4}) {
		t.Fatalf("inBuf: %+#v", buf)
	}
	// Read the second (last) segment.
	n, err = testIn.Read(buf)
	if n != 5 || err != nil {
		t.Fatalf("n: %+#v, err: %+#v", n, err)
	}
	if !reflect.DeepEqual(buf[:n], []byte{5, 6, 7, 8, 9}) {
		t.Fatalf("inBuf: %+#v", buf)
	}
	// Test read.
	n, err = testOut.Write([]byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 0})
	if n != 10 || err != nil {
		t.Fatalf("n: %+#v, err: %+#v", n, err)
	}
	// Drain the entire internal buffer in a single read.
	n, err = tc.Read(buf)
	if n != 10 || err != nil {
		t.Fatalf("n: %+#v, err: %+#v", n, err)
	}
	if !reflect.DeepEqual(buf, []byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 0}) {
		t.Fatalf("inBuf: %+#v", buf)
	}
}
