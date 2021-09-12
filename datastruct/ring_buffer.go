package datastruct

import (
	"sync"
)

// RingBuffer is a rudimentary implementation of a fixed-size circular buffer.
// All buffered elements are strings.
type RingBuffer struct {
	size    int64
	counter int64
	buf     []string
	mutex   *sync.RWMutex
}

// NewRingBuffer returns an initialised ring buffer.
func NewRingBuffer(size int64) *RingBuffer {
	if size < 1 {
		panic("NewRingBuffer: size must be greater than 0")
	}
	return &RingBuffer{
		size:  size,
		buf:   make([]string, size),
		mutex: new(sync.RWMutex),
	}
}

// Push places a new element into ring buffer.
func (r *RingBuffer) Push(elem string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.counter++
	r.buf[r.counter%r.size] = elem
}

/*
Clear sets all buffered elements to empty string, consequently GetAll function will return an empty string array
indicating there's no element.
*/
func (r *RingBuffer) Clear() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.buf = make([]string, r.size)
}

/*
IterateReverse traverses the ring buffer from the latest element to the oldest element.
The iterator function is fed an element value as sole parameter. If the function returns false, iteration is stopped
immediately. The total number of elements iterated is not predictable, and iteration loop always skips empty elements.
*/
func (r *RingBuffer) IterateReverse(fun func(string) bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
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
	r.mutex.RLock()
	defer r.mutex.RUnlock()
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
