package serialport

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestReadUntilDelimiter(t *testing.T) {
	src := bytes.NewReader([]byte("abc\r\n123\r\n\r\n"))
	if buf, err := readUntilDelimiter(src, '\r', '\n'); err != nil || !reflect.DeepEqual(buf, []byte("abc")) {
		t.Fatal(buf, err)
	}
	src = bytes.NewReader([]byte("\r\n123\r\n\r\n"))
	if buf, err := readUntilDelimiter(src, '\r', '\n'); err != nil || !reflect.DeepEqual(buf, []byte("123")) {
		t.Fatal(buf, err)
	}
	src = bytes.NewReader([]byte("\n123\n"))
	if buf, err := readUntilDelimiter(src, '\r', '\n'); err != nil || !reflect.DeepEqual(buf, []byte("123")) {
		t.Fatal(buf, err)
	}
	src = bytes.NewReader([]byte(""))
	if _, err := readUntilDelimiter(src, '\r', '\n'); err != io.EOF {
		t.Fatal(err)
	}
}

func TestWriteSlowly(t *testing.T) {
	fh, err := ioutil.TempFile("", "laitos-TestWriteSlowly")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(fh.Name())

	// Slowly write 5 seconds worth of bytes
	beginSec := time.Now().Unix()
	if err := writeSlowly(fh, bytes.Repeat([]byte{0}, 1000/WriteSlowlyIntervalMS*5)); err != nil {
		t.Fatal(err)
	}
	durationSec := time.Now().Unix() - beginSec
	if durationSec < 4 || durationSec > 7 {
		t.Fatal(durationSec)
	}
}
