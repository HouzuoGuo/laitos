package misc

import (
	"bytes"
	"io/ioutil"
	"net"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

func TestProbePort(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:10699")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	if !ProbePort(3*time.Second, "localhost", 10699) {
		t.Fatal("should have seen the listening port")
	}

	start := time.Now()
	if ProbePort(3*time.Second, "localhost", 23009) {
		t.Fatal("should not have seen an unoccupied port")
	}
	duration := time.Now().Sub(start)
	if duration > 5*time.Second {
		t.Fatalf("ProbePort took way too long")
	}
}

func TestPipeTCPConnection(t *testing.T) {
	writeTo, drainFrom := net.Pipe()
	defer writeTo.Close()
	defer drainFrom.Close()

	drainTo, readFrom := net.Pipe()
	defer drainTo.Close()
	defer readFrom.Close()

	go PipeConn(lalog.DefaultLogger, true, 1*time.Second, 1280, drainFrom, drainTo)
	go PipeConn(lalog.DefaultLogger, true, 1*time.Second, 1280, drainTo, drainFrom)

	data := bytes.Repeat([]byte{1}, 1024*1024)
	go func() {
		if length, err := writeTo.Write(data); length != len(data) || err != nil {
			lalog.DefaultLogger.Panic("", err, "unexpected length (%d) or err", length)
		}
	}()
	recv, err := ioutil.ReadAll(readFrom)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, recv) {
		t.Fatal("did not receive the data")
	}
}
