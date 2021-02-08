package datastruct

import (
	"fmt"
	"math"
	"sync"
)

// LeastRecentlyUsedBuffer implements the least-recently-used caching algorithm using an in-memory buffer.
// All buffered elements are strings.
type LeastRecentlyUsedBuffer struct {
	maxCapacity  int
	usageCounter uint64
	lastUsed     map[string]uint64
	mutex        *sync.RWMutex
}

// NewLeastRecentlyUsedBuffer returns an initialised LRU buffer.
func NewLeastRecentlyUsedBuffer(maxCapacity int) *LeastRecentlyUsedBuffer {
	if maxCapacity < 1 {
		panic("NewLeastRecentlyUsedBuffer: size must be greater than 0")
	}
	return &LeastRecentlyUsedBuffer{
		maxCapacity:  maxCapacity,
		usageCounter: 0,
		lastUsed:     make(map[string]uint64),
		mutex:        new(sync.RWMutex),
	}
}

// Add the element into LRU buffer.
// If due to capacity reasons the oldest element was evicted in doing so, then return the evited element.
func (lru *LeastRecentlyUsedBuffer) Add(elem string) (alreadyPresent bool, evicted string) {
	lru.mutex.Lock()
	defer lru.mutex.Unlock()
	lru.usageCounter++
	if _, present := lru.lastUsed[elem]; present {
		// Increase the last used
		lru.lastUsed[elem] = lru.usageCounter
		alreadyPresent = true
	} else {
		if len(lru.lastUsed) == lru.maxCapacity {
			// Find the oldest element
			var oldestElem string
			oldestCounter := uint64(math.MaxUint64)
			for elem, lastUsed := range lru.lastUsed {
				if lastUsed < uint64(oldestCounter) {
					oldestElem = elem
					oldestCounter = lastUsed
				}
			}
			// Evict the oldest element by deleting it
			delete(lru.lastUsed, oldestElem)
			evicted = oldestElem
		}
		// Put the new element into buffer
		lru.lastUsed[elem] = lru.usageCounter
	}
	return
}

// Contains returns true only if the element is currently in the LRU buffer.
func (lru *LeastRecentlyUsedBuffer) Contains(elem string) bool {
	lru.mutex.RLock()
	defer lru.mutex.RUnlock()
	_, exists := lru.lastUsed[elem]
	return exists
}

// Remove the elemnt from LRU buffer, freeing up a unit of capacity for more elements.
func (lru *LeastRecentlyUsedBuffer) Remove(elem string) {
	lru.mutex.Lock()
	defer lru.mutex.Unlock()
	delete(lru.lastUsed, elem)
}

// Len returns the number of elements currently kept in the buffer.
func (lru *LeastRecentlyUsedBuffer) Len() int {
	lru.mutex.RLock()
	defer lru.mutex.RUnlock()
	return len(lru.lastUsed)
}

func (lru *LeastRecentlyUsedBuffer) String() string {
	return fmt.Sprintf("%+v", lru.lastUsed)
}
