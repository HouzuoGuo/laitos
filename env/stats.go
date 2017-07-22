package env

import (
	"fmt"
	"sync"
)

// Stats collect counter and aggregated numeric data from a stream of triggers.
type Stats struct {
	count uint64      // count is the number of times trigger has occured.
	mutex *sync.Mutex // mutex protects structure from concurrent modifications

	lowest, highest, average, total float64
}

// NewStats returns an initialised stats structure.
func NewStats() *Stats {
	return &Stats{
		mutex: new(sync.Mutex),
	}
}

// Trigger increases counter by one and places the input quantity into numeric statistics.
func (s *Stats) Trigger(qty float64) {
	s.mutex.Lock()
	if s.lowest == 0 || s.lowest > qty {
		s.lowest = qty
	}
	if s.highest == 0 || s.highest < qty {
		s.highest = qty
	}
	s.average = (s.average*float64(s.count) + qty) / (float64(s.count) + 1.0)
	s.total += qty
	s.count++
	s.mutex.Unlock()
}

// GetStats returns the latest counter and stats numbers.
func (s *Stats) GetStats() (lowest, highest, average, total float64, count uint64) {
	return s.lowest, s.highest, s.average, s.total, s.count
}

// Format returns all stats formatted into a single line of string.
func (s *Stats) Format(numDecimals int) string {
	format := fmt.Sprintf("%%.%df/%%.%df/%%.%df/%%.%df(%%d)", numDecimals, numDecimals, numDecimals, numDecimals)
	return fmt.Sprintf(format, s.lowest, s.average, s.highest, s.total, s.count)
}
