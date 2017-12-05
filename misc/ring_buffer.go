package misc

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
IterateReverse traverses the ring buffer from the latest element to the oldest element.
The iterator function is fed an element value as sole parameter. If the function returns false, iteration is stopped
immediately. The total number of elements iterated is not predictable, and iteration loop always skips empty elements.
*/
func (r *RingBuffer) IterateReverse(fun func(string) bool) {
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

// GetAll returns all elements (oldest to the latest) of the ring buffer in a string array.
func (r *RingBuffer) GetAll() (ret []string) {
	reversed := make([]string, 0, r.size)
	r.IterateReverse(func(elem string) bool {
		reversed = append(reversed, elem)
		return true
	})

	ret = make([]string, len(reversed))
	for i, s := range reversed {
		ret[len(ret)-1-i] = s
	}
	return
}
