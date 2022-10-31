package dnsd

import (
	"testing"
)

func TestMaxUpstreamSegmentLength(t *testing.T) {
	got := MaxUpstreamSegmentLength("example.com")
	want := 146
	if got != want {
		t.Fatalf("got: %v, want: %v", got, want)
	}
}

func TestMaxDownstreamSegmentLengthTXT(t *testing.T) {
	got := MaxDownstreamSegmentLengthTXT("example.com")
	want := 684
	if got != want {
		t.Fatalf("got: %v, want: %v", got, want)
	}
}
