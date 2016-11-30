package main

import (
	"testing"
	"time"
)

func TestPauseFewSecs(t *testing.T) {
	now := time.Now().Unix()
	PauseFewSecs()
	diff1 := time.Now().Unix() - now
	if diff1 < 2 || diff1 > 4 {
		t.Fatal(diff1)
	}
}
