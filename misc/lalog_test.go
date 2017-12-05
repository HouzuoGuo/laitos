package misc

import (
	"errors"
	"strings"
	"testing"
)

func TestLogger_Format(t *testing.T) {
	logger := Logger{}
	if msg := logger.Format("", "", nil, "a"); msg != "a" {
		t.Fatal(msg)
	}
	if msg := logger.Format("", "", errors.New("test"), "a"); msg != "Error \"test\" - a" {
		t.Fatal(msg)
	}
	if msg := logger.Format("", "act", errors.New("test"), "a"); msg != "(act): Error \"test\" - a" {
		t.Fatal(msg)
	}
	if msg := logger.Format("fun", "act", errors.New("test"), "a"); msg != "fun(act): Error \"test\" - a" {
		t.Fatal(msg)
	}
	logger.ComponentID = "ha"
	if msg := logger.Format("fun", "act", errors.New("test"), "a"); msg != "[ha].fun(act): Error \"test\" - a" {
		t.Fatal(msg)
	}
	logger.ComponentName = "comp"
	if msg := logger.Format("fun", "act", errors.New("test"), "a"); msg != "comp[ha].fun(act): Error \"test\" - a" {
		t.Fatal(msg)
	}
	if msg := logger.Format("fun", "act", errors.New("test"), strings.Repeat("a", MaxLogMessageLen)); len(msg) != MaxLogMessageLen {
		t.Fatal(msg)
	}
}

func TestLogger_Panicf(t *testing.T) {
	defer func() {
		recover()
	}()
	logger := Logger{}
	logger.Panicf("", "", nil, "")
	t.Fatal("did not panic")
}

func TestLogger_Printf(t *testing.T) {
	logger := Logger{}
	logger.Printf("", "", nil, "")
	logger.Printf("", "", nil, "")

	var countLog, countWarn int
	LatestLogs.IterateReverse(func(_ string) bool {
		countLog++
		return true
	})
	LatestWarnings.IterateReverse(func(_ string) bool {
		countWarn++
		return true
	})
	if countLog != 2 {
		t.Fatal(countLog, countWarn)
	}

	logger.Printf("", "", errors.New(""), "")
	logger.Printf("", "", errors.New(""), "")

	countLog = 0
	countWarn = 0
	LatestLogs.IterateReverse(func(_ string) bool {
		countLog++
		return true
	})
	LatestWarnings.IterateReverse(func(_ string) bool {
		countWarn++
		return true
	})
	// Depending on the test case execution order, the count may be higher if Warningf test has already run.
	if countLog < 4 || countWarn < 2 {
		t.Fatal(countLog, countWarn)
	}

}

func TestLogger_Warningf(t *testing.T) {
	logger := Logger{}
	logger.Warningf("", "", nil, "")
	logger.Warningf("", "", nil, "")

	var countLog, countWarn int
	LatestLogs.IterateReverse(func(_ string) bool {
		countLog++
		return true
	})
	LatestWarnings.IterateReverse(func(_ string) bool {
		countWarn++
		return true
	})
	// Depending on the test case execution order, the count may be higher if Printf test has already run.
	if countLog < 2 || countWarn < 2 {
		t.Fatal(countLog, countWarn)
	}

	logger.Warningf("", "", errors.New(""), "")
	logger.Warningf("", "", errors.New(""), "")

	countWarn = 0
	countLog = 0
	LatestLogs.IterateReverse(func(_ string) bool {
		countLog++
		return true
	})
	LatestWarnings.IterateReverse(func(_ string) bool {
		countWarn++
		return true
	})
	// Depending on the test case execution order, the count may be higher if Printf test has already run.
	if countLog < 4 || countWarn < 4 {
		t.Fatal(countLog, countWarn)
	}
}
