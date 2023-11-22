package misc

import (
	"fmt"
	"sync"
)

const StatsSumaryFormat = "%%.%df/%%.%df/%%.%df,%%.%df(%%d)"

var DefaultStatsDisplayFormat = StatsDisplayFormat{
	DivisionFactor: 1,
	NumDecimals:    9,
}

// StatsDisplayFormat determines the human-readable scale and formatting of an instance of stats counter.
type StatsDisplayFormat struct {
	DivisionFactor float64
	NumDecimals    int
}

// StatsDisplayValue is the human-readable transformation of an instance of stats counter.
type StatsDisplayValue struct {
	Lowest  float64
	Average float64
	Highest float64
	Total   float64
	Count   uint64
	Summary string
}

// Stats collect counter and aggregated numeric data from a stream of triggers.
type Stats struct {
	count                           uint64        // count is the number of times trigger has occurred.
	mutex                           *sync.RWMutex // mutex protects structure from concurrent modifications.
	lowest, highest, average, total float64

	DisplayFormat StatsDisplayFormat
}

// NewStats returns an initialised stats structure.
func NewStats(displayFormat StatsDisplayFormat) *Stats {
	if displayFormat.DivisionFactor == 0 && displayFormat.NumDecimals == 0 {
		displayFormat = DefaultStatsDisplayFormat
	}
	return &Stats{
		mutex:         new(sync.RWMutex),
		DisplayFormat: displayFormat,
	}
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
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return int(s.count)
}

// Format returns all stats formatted into a single line of string after the numbers (excluding counter) are divided by the factor.
func (s *Stats) Format() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	format := fmt.Sprintf(StatsSumaryFormat, s.DisplayFormat.NumDecimals, s.DisplayFormat.NumDecimals, s.DisplayFormat.NumDecimals, s.DisplayFormat.NumDecimals)
	return fmt.Sprintf(format, s.lowest/s.DisplayFormat.DivisionFactor, s.average/s.DisplayFormat.DivisionFactor, s.highest/s.DisplayFormat.DivisionFactor, s.total/s.DisplayFormat.DivisionFactor, s.count)
}

// DisplayValue returns the human-readable display values of the raw stats counter instance.
func (s *Stats) DisplayValue() StatsDisplayValue {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	format := fmt.Sprintf(StatsSumaryFormat, s.DisplayFormat.NumDecimals, s.DisplayFormat.NumDecimals, s.DisplayFormat.NumDecimals, s.DisplayFormat.NumDecimals)
	return StatsDisplayValue{
		Lowest:  s.lowest / s.DisplayFormat.DivisionFactor,
		Average: s.average / s.DisplayFormat.DivisionFactor,
		Highest: s.average / s.DisplayFormat.DivisionFactor,
		Total:   s.total / s.DisplayFormat.DivisionFactor,
		Count:   s.count,
		Summary: fmt.Sprintf(format, s.lowest/s.DisplayFormat.DivisionFactor, s.average/s.DisplayFormat.DivisionFactor, s.highest/s.DisplayFormat.DivisionFactor, s.total/s.DisplayFormat.DivisionFactor, s.count),
	}
}
