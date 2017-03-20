package lalog

import (
	"reflect"
	"testing"
)

func TestRingBuffer_Push(t *testing.T) {
	retrievedIndexes := make([]uint64, 0, 10)
	retrievedValues := make([]string, 0, 10)

	iter := func(index uint64, value string) bool {
		retrievedIndexes = append(retrievedIndexes, index)
		retrievedValues = append(retrievedValues, value)
		return true
	}

	r := NewRingBuffer(2)

	r.Push("0")
	retrievedIndexes = make([]uint64, 0, 10)
	retrievedValues = make([]string, 0, 10)
	r.Iterate(iter)
	if !reflect.DeepEqual(retrievedIndexes, []uint64{0}) {
		t.Fatal(retrievedIndexes)
	}
	if !reflect.DeepEqual(retrievedValues, []string{"0"}) {
		t.Fatal(retrievedValues)
	}

	r.Push("1")
	retrievedIndexes = make([]uint64, 0, 10)
	retrievedValues = make([]string, 0, 10)
	r.Iterate(iter)
	if !reflect.DeepEqual(retrievedIndexes, []uint64{0, 1}) {
		t.Fatal(retrievedIndexes)
	}
	if !reflect.DeepEqual(retrievedValues, []string{"0", "1"}) {
		t.Fatal(retrievedValues)
	}

	r.Push("2")
	retrievedIndexes = make([]uint64, 0, 10)
	retrievedValues = make([]string, 0, 10)
	r.Iterate(iter)
	if !reflect.DeepEqual(retrievedIndexes, []uint64{0, 1}) {
		t.Fatal(retrievedIndexes)
	}
	if !reflect.DeepEqual(retrievedValues, []string{"1", "2"}) {
		t.Fatal(retrievedValues)
	}

	r.Push("3")
	retrievedIndexes = make([]uint64, 0, 10)
	retrievedValues = make([]string, 0, 10)
	r.Iterate(iter)
	if !reflect.DeepEqual(retrievedIndexes, []uint64{0, 1}) {
		t.Fatal(retrievedIndexes)
	}
	if !reflect.DeepEqual(retrievedValues, []string{"2", "3"}) {
		t.Fatal(retrievedValues)
	}
}
