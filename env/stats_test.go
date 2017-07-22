package env

import (
	"math"
	"testing"
)

func TestStats(t *testing.T) {
	s := NewStats(0.5)
	if l, h, a, to, c := s.GetStats(); l != 0 || h != 0 || a != 0 || to != 0 || c != 0 {
		t.Fatal(l, h, a, to, c)
	}
	// Add three triggers, all of which are above lowest threshold.
	s.Trigger(1.0)
	if l, h, a, to, c := s.GetStats(); l != 1 || h != 1 || a != 1 || to != 1 || c != 1 {
		t.Fatal(l, h, a, to, c)
	}
	s.Trigger(9.0)
	if l, h, a, to, c := s.GetStats(); l != 1 || h != 9 || a != 5 || to != 10 || c != 2 {
		t.Fatal(l, h, a, to, c)
	}
	s.Trigger(2.0)
	if l, h, a, to, c := s.GetStats(); l != 1 || h != 9 || a != 4 || to != 12 || c != 3 {
		t.Fatal(l, h, a, to, c)
	}
	// Trigger is below threshold and should not be registered in lowest value
	s.Trigger(0.4)
	if l, h, a, to, c := s.GetStats(); l != 1 || h != 9 || math.Abs(a-3.1) > 0.001 || math.Abs(to-12.4) > 0.001 || c != 4 {
		t.Fatal(l, h, a, to, c)
	}
}
