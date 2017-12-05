package misc

import (
	"reflect"
	"testing"
)

func TestRingBuffer_Push(t *testing.T) {
	r := NewRingBuffer(2)
	r.Push("0")
	if !reflect.DeepEqual(r.GetAll(), []string{"0"}) {
		t.Fatal(r.GetAll())
	}

	r.Push("1")
	if !reflect.DeepEqual(r.GetAll(), []string{"0", "1"}) {
		t.Fatal(r.GetAll())
	}

	r.Push("2")
	if !reflect.DeepEqual(r.GetAll(), []string{"1", "2"}) {
		t.Fatal(r.GetAll())
	}

	r.Push("3")
	if !reflect.DeepEqual(r.GetAll(), []string{"2", "3"}) {
		t.Fatal(r.GetAll())
	}
}
