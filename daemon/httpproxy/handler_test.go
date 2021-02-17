package httpproxy

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

func TestPipeTCPConnection(t *testing.T) {
	writeTo, drainFrom := net.Pipe()
	defer writeTo.Close()
	defer drainFrom.Close()

	drainTo, readFrom := net.Pipe()
	defer drainTo.Close()
	defer readFrom.Close()

	go PipeTCPConnection(*lalog.DefaultLogger, 1*time.Second, drainFrom, drainTo)
	go PipeTCPConnection(*lalog.DefaultLogger, 1*time.Second, drainTo, drainFrom)

	data := bytes.Repeat([]byte{1}, 1024*1024)
	go func() {
		if length, err := writeTo.Write(data); length != len(data) || err != nil {
			lalog.DefaultLogger.Panic("", "", err, "unexpected length (%d) or err", length)
		}
	}()
	recv, err := io.ReadAll(readFrom)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, recv) {
		t.Fatal("did not receive the data")
	}
}
