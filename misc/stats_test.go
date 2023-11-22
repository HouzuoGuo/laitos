package misc

import (
	"reflect"
	"testing"
)

func TestStats(t *testing.T) {
	s := NewStats(StatsDisplayFormat{
		DivisionFactor: 10,
		NumDecimals:    2,
	})
	if s.lowest != 0 || s.highest != 0 || s.average != 0 || s.total != 0 || s.count != 0 {
		t.Fatalf("%+v", s)
	}
	// Invalid trigger quantity should not affect stats
	s.Trigger(-1.0)
	if s.lowest != 0 || s.highest != 0 || s.average != 0 || s.total != 0 || s.count != 0 {
		t.Fatalf("%+v", s)
	}
	// Quantity of 0 goes into count
	s.Trigger(0.0)
	if s.lowest != 0 || s.highest != 0 || s.average != 0 || s.total != 0 || s.count != 1 {
		t.Fatalf("%+v", s)
	}
	// Three valid trigger quantities
	s.Trigger(1.0)
	if s.lowest != 1 || s.highest != 1 || s.average != 0.5 || s.total != 1 || s.count != 2 {
		t.Fatalf("%+v", s)
	}
	s.Trigger(5.0)
	if s.lowest != 1 || s.highest != 5 || s.average != 2 || s.total != 6 || s.count != 3 {
		t.Fatalf("%+v", s)
	}
	s.Trigger(6.0)
	if s.lowest != 1 || s.highest != 6 || s.average != 3 || s.total != 12 || s.count != 4 {
		t.Fatalf("%+v", s)
	}
	if s.Count() != 4 {
		t.Fatal(s.Count())
	}
	t.Run("format to text", func(t *testing.T) {
		if str := s.Format(); str != "0.10/0.30/0.60,1.20(4)" {
			t.Fatalf(str)
		}
	})
	t.Run("convert to display values", func(t *testing.T) {
		want := StatsDisplayValue{
			Lowest:  0.1,
			Average: 0.3,
			Highest: 0.3,
			Total:   1.2,
			Count:   4,
			Summary: "0.10/0.30/0.60,1.20(4)",
		}
		got := s.DisplayValue()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got: %+v, want: %+v", got, want)
		}
	})
}
