package misc

import (
	"strings"
	"testing"
)

func TestGetLatestStats(t *testing.T) {
	for i := 0; i < 1928; i++ {
		AutoUnlockStats.Trigger(1)
	}
	if s := GetLatestStats(); !strings.Contains(s, "1928") {
		t.Fatal(s)
	}
}
