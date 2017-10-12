package misc

import (
	"testing"
)

func TestStats(t *testing.T) {
	s := NewStats()
	if s.lowest != 0 || s.highest != 0 || s.average != 0 || s.total != 0 || s.count != 0 {
		t.Fatalf("%+v", s)
	}
	// Invalid trigger quantity should not affect stats
	s.Trigger(-1.0)
	s.Trigger(0.0)
	if s.lowest != 0 || s.highest != 0 || s.average != 0 || s.total != 0 || s.count != 0 {
		t.Fatalf("%+v", s)
	}
	// Three valid trigger quantities
	s.Trigger(1.0)
	if s.lowest != 1 || s.highest != 1 || s.average != 1 || s.total != 1 || s.count != 1 {
		t.Fatalf("%+v", s)
	}
	s.Trigger(9.0)
	if s.lowest != 1 || s.highest != 9 || s.average != 5 || s.total != 10 || s.count != 2 {
		t.Fatalf("%+v", s)
	}
	s.Trigger(2.0)
	if s.lowest != 1 || s.highest != 9 || s.average != 4 || s.total != 12 || s.count != 3 {
		t.Fatalf("%+v", s)
	}
	// Format test
	if str := s.Format(10, 2); str != "0.10/0.40/0.90,1.20(3)" {
		t.Fatalf(str)
	}
}
