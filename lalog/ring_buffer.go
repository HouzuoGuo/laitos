package lalog

import (
	"sync/atomic"
)

// Implement a ring buffer of strings, tailored to store latest log entries.
type RingBuffer struct {
	size    int64
	counter int64
	buf     []string
}

// Pre-allocate string buffer of the specified size and initialise ring buffer.
func NewRingBuffer(size int64) *RingBuffer {
	if size < 1 {
		panic("NewRingBuffer: size must be greater than 0")
	}
	return &RingBuffer{
		size: size,
		buf:  make([]string, size),
	}
}

// Add a new element into ring buffer.
func (r *RingBuffer) Push(elem string) {
	elemIndex := atomic.AddInt64(&r.counter, 1)
	r.buf[elemIndex%r.size] = elem
}

/*
Iterate through the ring buffer, beginning from the latest element through to the oldest element.
The iterator function is fed an element value as sole parameter. If the function returns false, iteration is stopped
immediately. The total number of elements iterated is not predictable, and iteration loop always skips empty elements.
*/
func (r *RingBuffer) Iterate(fun func(string) bool) {
	currentIndex := r.counter % r.size
	for i := currentIndex; i >= 0; i-- {
		value := r.buf[i]
		if value != "" {
			if !fun(value) {
				return
			}
		}
	}
	for i := r.size - 1; i > currentIndex; i-- {
		value := r.buf[i]
		if value != "" {
			if !fun(value) {
				return
			}
		}
	}
}
