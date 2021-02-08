package datastruct

import (
	"strconv"
	"testing"
)

func TestLeastRecentlyUsedBuffer(t *testing.T) {
	lru := NewLeastRecentlyUsedBuffer(3)
	// Fill the buffer up
	for i := 0; i < 3; i++ {
		alreadyPresent, evicted := lru.Add(strconv.Itoa(i))
		if alreadyPresent || evicted != "" {
			t.Fatal(alreadyPresent, evicted)
		}
		if !lru.Contains(strconv.Itoa(i)) {
			t.Fatal("element went missing", i)
		}
	}
	if len(lru.lastUsed) != 3 {
		t.Fatalf("unexpected buffer elements: %+v", lru.lastUsed)
	}

	// Continue adding elements without evicting present elements
	for i := 0; i < 3; i++ {
		alreadyPresent, evicted := lru.Add(strconv.Itoa(i))
		if !alreadyPresent || evicted != "" {
			t.Fatal(alreadyPresent, evicted)
		}
		if !lru.Contains(strconv.Itoa(i)) {
			t.Fatal("element went missing", i)
		}
	}
	if len(lru.lastUsed) != 3 {
		t.Fatalf("unexpected buffer elements: %+v", lru.lastUsed)
	}

	// Evict old elements by adding new elements
	for i := 3; i < 6; i++ {
		alreadyPresent, evicted := lru.Add(strconv.Itoa(i))
		// Should have evicted from the oldest (0) to the latest (2)
		if alreadyPresent || evicted != strconv.Itoa(i-3) {
			t.Fatal(alreadyPresent, evicted)
		}
		if !lru.Contains(strconv.Itoa(i)) {
			t.Fatal("element went missing", i)
		}
	}
	if len(lru.lastUsed) != 3 {
		t.Fatalf("unexpected buffer elements: %+v", lru.lastUsed)
	}

	// Evice old elements in non-sequential order
	// LRU now has 3, 4, 5. Make the buffered elements 3, 5, 8 by adding 2, 5, 3, 8
	var evictions = []struct {
		add, evicted string
	}{
		{"2", "3"},
		{"5", ""},
		{"3", "4"},
		{"8", "2"},
	}
	for _, addAndEvict := range evictions {
		if _, evicted := lru.Add(addAndEvict.add); evicted != addAndEvict.evicted {
			t.Fatalf("expected to add %s and evict %s, though actually evicted %s", addAndEvict.add, addAndEvict.evicted, evicted)
		}
	}
	for _, addAndEvict := range evictions[len(evictions)-lru.maxCapacity:] {
		if !lru.Contains(addAndEvict.add) {
			t.Fatal("element went missing", addAndEvict.add)
		}
	}
}
