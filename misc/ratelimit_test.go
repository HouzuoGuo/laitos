package misc

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestRateLimit(t *testing.T) {
	// Log spam reduction
	limit := RateLimit{UnitSecs: 1, MaxCount: 23}
	limit.Initialise()
	if limit.UnitSecs != 1 || limit.MaxCount != 23 {
		t.Fatalf("%+v", limit)
	}

	limit = RateLimit{UnitSecs: 1, MaxCount: 22}
	limit.Initialise()
	if limit.UnitSecs != 11 || limit.MaxCount != 22*11 {
		t.Fatalf("%+v", limit)
	}

	limit = RateLimit{UnitSecs: 1, MaxCount: 21}
	limit.Initialise()
	if limit.UnitSecs != 7 || limit.MaxCount != 21*7 {
		t.Fatalf("%+v", limit)
	}

	limit = RateLimit{UnitSecs: 3, MaxCount: 4}
	limit.Initialise()
	// Three actors should get two chances each
	success := [3]int{}
	successMutex := new(sync.Mutex)
	for i := 0; i < 3; i++ {
		go func(i int) {
			for j := 0; j < 100; j++ {
				if limit.Add(strconv.Itoa(i), true) {
					successMutex.Lock()
					success[i]++
					successMutex.Unlock()
				}
			}
		}(i)
	}
	time.Sleep(1 * time.Second)
	for i := 0; i < 3; i++ {
		successMutex.Lock()
		if success[i] != 4 {
			t.Fatal(success)
		}
		successMutex.Unlock()
	}
}

func TestRateLimit2(t *testing.T) {
	success := [3]int{}
	successMutex := new(sync.Mutex)
	// Test rate limit over the period of 15 seconds
	limit := RateLimit{UnitSecs: 3, MaxCount: 4}
	limit.Initialise()
	for i := 0; i < 3; i++ {
		successMutex.Lock()
		success[i] = 0
		successMutex.Unlock()
		go func(i int) {
			// Will finish in exactly 0.6*25=15 seconds
			for j := 0; j < 25; j++ {
				if limit.Add(strconv.Itoa(i), true) {
					successMutex.Lock()
					success[i]++
					successMutex.Unlock()
				}
				time.Sleep(600 * time.Millisecond)
			}
		}(i)
	}
	time.Sleep(17 * time.Second)
	successMutex.Lock()
	for i := 0; i < 3; i++ {
		if success[i] > 22 || success[i] < 20 {
			t.Fatal(success)
		}
	}
	successMutex.Unlock()
}
