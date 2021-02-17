package serialport

import (
	"bytes"
	"io"
	"reflect"
	"testing"
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
