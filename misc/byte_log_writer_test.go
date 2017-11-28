package misc

import (
	"bytes"
	"reflect"
	"testing"
)

func TestByteLogWriter(t *testing.T) {
	null := new(bytes.Buffer)
	writer := NewByteLogWriter(null, 5)
	// Plenty of room
	if _, err := writer.Write([]byte{0, 1}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(writer.Retrieve(), []byte{0, 1}) {
		t.Fatal(writer.Retrieve())
	}
	// Exactly full
	if _, err := writer.Write([]byte{2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(writer.Retrieve(), []byte{0, 1, 2, 3, 4}) {
		t.Fatal(writer.Retrieve())
	}
	// Overwriting older bytes
	if _, err := writer.Write([]byte{5, 6}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(writer.Retrieve(), []byte{2, 3, 4, 5, 6}) {
		t.Fatal(writer.Retrieve())
	}
	// Overwriting entire memory several times (789, 01234, 56789)
	if _, err := writer.Write([]byte{7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(writer.Retrieve(), []byte{5, 6, 7, 8, 9}) {
		t.Fatal(writer.Retrieve())
	}
	// Small write again
	if _, err := writer.Write([]byte{0, 1, 2}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(writer.Retrieve(), []byte{8, 9, 0, 1, 2}) {
		t.Fatal(writer.Retrieve())
	}
}
