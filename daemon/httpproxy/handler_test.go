package httpproxy

import (
	"bytes"
	"io/ioutil"
	"net"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

func TestPipeTCPConnection(t *testing.T) {
	writeTo, drainFrom := net.Pipe()
	defer writeTo.Close()
	defer drainFrom.Close()

	drainTo, readFrom := net.Pipe()
	defer drainTo.Close()
	defer readFrom.Close()

	go misc.PipeConn(*lalog.DefaultLogger, true, 1*time.Second, 1280, drainFrom, drainTo)
	go misc.PipeConn(*lalog.DefaultLogger, true, 1*time.Second, 1280, drainTo, drainFrom)

	data := bytes.Repeat([]byte{1}, 1024*1024)
	go func() {
		if length, err := writeTo.Write(data); length != len(data) || err != nil {
			lalog.DefaultLogger.Panic("", "", err, "unexpected length (%d) or err", length)
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
