package lalog

import (
	"sync/atomic"
)

// Implement a ring buffer of strings, tailored to store latest log entries.
type RingBuffer struct {
	size    uint64
	counter uint64
	buf     []string
}

// Pre-allocate string buffer of the specified size and initialise ring buffer.
func NewRingBuffer(size uint64) *RingBuffer {
	return &RingBuffer{
		size: size,
		buf:  make([]string, size),
	}
}

// Add a new element into ring buffer.
func (r *RingBuffer) Push(elem string) {
	elemIndex := atomic.AddUint64(&r.counter, 1)
	r.buf[elemIndex%r.size] = elem
}

/*
Iterate through the ring buffer, beginning from the oldest element through to the current element.
The iterator function is fed an element index [0, size-1], and the corresponding element value. If the function
returns false, iteration is stopped immediately. The total number of elements iterated is not predictable, and iteration
loop always skips empty elements.
*/
func (r *RingBuffer) Iterate(fun func(uint64, string) bool) {
	var iterIndex uint64 = 0
	counterIndex := r.counter + 1
	for counterEnd := counterIndex + r.size; counterIndex < counterEnd; counterIndex++ {
		value := r.buf[counterIndex%r.size]
		if value != "" {
			if !fun(iterIndex, value) {
				return
			}
			iterIndex++
		}
	}
}
