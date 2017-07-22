package env

import "testing"

func TestStats(t *testing.T) {
	s := NewStats()
	if l, h, a, to, c := s.GetStats(); l != 0 || h != 0 || a != 0 || to != 0 || c != 0 {
		t.Fatal(l, h, a, to, c)
	}
	s.Trigger(1)
	if l, h, a, to, c := s.GetStats(); l != 1 || h != 1 || a != 1 || to != 1 || c != 1 {
		t.Fatal(l, h, a, to, c)
	}
	s.Trigger(9)
	if l, h, a, to, c := s.GetStats(); l != 1 || h != 9 || a != 5 || to != 10 || c != 2 {
		t.Fatal(l, h, a, to, c)
	}
	s.Trigger(2)
	if l, h, a, to, c := s.GetStats(); l != 1 || h != 9 || a != 4 || to != 12 || c != 3 {
		t.Fatal(l, h, a, to, c)
	}
}
