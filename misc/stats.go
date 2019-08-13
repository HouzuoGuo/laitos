package misc

import (
	"fmt"
	"sync"
)

// Stats collect counter and aggregated numeric data from a stream of triggers.
type Stats struct {
	count uint64      // count is the number of times trigger has occurred.
	mutex *sync.Mutex // mutex protects structure from concurrent modifications.

	lowest, highest, average, total float64
}

// NewStats returns an initialised stats structure.
func NewStats() *Stats {
	return &Stats{mutex: new(sync.Mutex)}
}

// Trigger increases counter by one and places the input quantity into numeric statistics.
func (s *Stats) Trigger(qty float64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if qty < 0 {
		// Invalid quantity
		return
	}
	s.count++
	if qty == 0 {
		// Interval is too small for updating high/low/average
		return
	}
	if s.highest == 0 || s.highest < qty {
		s.highest = qty
	}
	if s.lowest == 0 || s.lowest > qty {
		s.lowest = qty
	}
	s.total += qty
	s.average = s.total / float64(s.count)
}

// Count returns the verbatim counter value, that is the number of times some action has triggered.
func (s *Stats) Count() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return int(s.count)
}

// Format returns all stats formatted into a single line of string after the numbers (excluding counter) are divided by the factor.
func (s *Stats) Format(divisionFactor float64, numDecimals int) string {
	format := fmt.Sprintf("%%.%df/%%.%df/%%.%df,%%.%df(%%d)", numDecimals, numDecimals, numDecimals, numDecimals)
	return fmt.Sprintf(format, s.lowest/divisionFactor, s.average/divisionFactor, s.highest/divisionFactor, s.total/divisionFactor, s.count)
}
