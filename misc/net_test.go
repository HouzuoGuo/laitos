package misc

import (
	"bytes"
	"io"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestProbePort(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:10699")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	if !ProbePort(1*time.Second, "localhost", 10699) {
		t.Fatal("should have seen the listening port")
	}

	start := time.Now()
	if ProbePort(1*time.Second, "localhost", 23009) {
		t.Fatal("should not have seen an unoccupied port")
	}
	duration := time.Now().Sub(start)
	if duration > 1100*time.Millisecond {
		t.Fatalf("ProbePort took way too long")
	}
}

func TestPipe(t *testing.T) {
	src := []byte{0, 1, 2, 3, 4, 5}
	var dest bytes.Buffer
	if err := Pipe(2, bytes.NewReader(src), &dest); err != io.EOF {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dest.Bytes(), src) {
		t.Fatalf("got: %+#v, want: %+#v", dest.Bytes(), src)
	}
}
