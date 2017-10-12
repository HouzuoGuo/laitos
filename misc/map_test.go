package misc

import (
	"sync"
	"testing"
)

func TestLenSyncMap(t *testing.T) {
	m := new(sync.Map)
	if l := LenSyncMap(m); l != 0 {
		t.Fatal(l)
	}
	m.Store(1, "a")
	m.Store(2, "b")
	m.Store(3, "c")
	if l := LenSyncMap(m); l != 3 {
		t.Fatal(l)
	}
}
