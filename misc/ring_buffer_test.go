package misc

import (
	"reflect"
	"testing"
)

func TestRingBuffer_Push(t *testing.T) {
	retrievedValues := make([]string, 0, 10)
	iter := func(value string) bool {
		retrievedValues = append(retrievedValues, value)
		return true
	}

	r := NewRingBuffer(2)
	r.Push("0")
	retrievedValues = make([]string, 0, 10)
	r.Iterate(iter)
	if !reflect.DeepEqual(retrievedValues, []string{"0"}) {
		t.Fatal(retrievedValues)
	}

	r.Push("1")
	retrievedValues = make([]string, 0, 10)
	r.Iterate(iter)
	if !reflect.DeepEqual(retrievedValues, []string{"1", "0"}) {
		t.Fatal(retrievedValues)
	}

	r.Push("2")
	retrievedValues = make([]string, 0, 10)
	r.Iterate(iter)
	if !reflect.DeepEqual(retrievedValues, []string{"2", "1"}) {
		t.Fatal(retrievedValues)
	}

	r.Push("3")
	retrievedValues = make([]string, 0, 10)
	r.Iterate(iter)
	if !reflect.DeepEqual(retrievedValues, []string{"3", "2"}) {
		t.Fatal(retrievedValues)
	}
}
