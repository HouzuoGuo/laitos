package misc

import (
	"strings"
	"testing"
)

func TestGetLatestStats(t *testing.T) {
	for range 1928 {
		AutoUnlockStats.Trigger(1)
	}
	if s := GetLatestStats(); !strings.Contains(s, "1928") {
		t.Fatal(s)
	}
}
