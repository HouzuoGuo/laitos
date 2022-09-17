package dnsclient

import (
	"testing"
)

func TestOptimalSegLen(t *testing.T) {
	got := OptimalSegLen("example.com")
	want := 143
	if got != want {
		t.Fatalf("got: %v, want: %v", got, want)
	}
}
