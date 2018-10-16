package lalog

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
	logger.ComponentID = []LoggerIDField{{"a", 1}, {"b", "c"}}
	if msg := logger.Format("fun", "act", errors.New("test"), "a"); msg != "[a=1;b=c].fun(act): Error \"test\" - a" {
		t.Fatal(msg)
	}
	logger.ComponentName = "comp"
	if msg := logger.Format("fun", "act", errors.New("test"), "a"); msg != "comp[a=1;b=c].fun(act): Error \"test\" - a" {
		t.Fatal(msg)
	}
	if msg := logger.Format("fun", "act", errors.New("test"), strings.Repeat("a", MaxLogMessageLen)); len(msg) != MaxLogMessageLen || !strings.Contains(msg, strings.Repeat("a", 1000)) {
		t.Fatal(len(msg), msg)
	}
}

func TestLogger_Panicf(t *testing.T) {
	defer func() {
		recover()
	}()
	logger := Logger{}
	logger.Panic("", "", nil, "")
	t.Fatal("did not panic")
}

func TestLogger_Printf(t *testing.T) {
	logger := Logger{}
	logger.Info("", "", nil, "")
	logger.Info("", "", nil, "")

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

	logger.Info("", "", errors.New(""), "")
	logger.Info("", "", errors.New(""), "")

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
	// Depending on the test case execution order, the count may be higher if Warning test has already run.
	if countLog < 4 || countWarn < 2 {
		t.Fatal(countLog, countWarn)
	}

}

func TestLogger_Warningf(t *testing.T) {
	logger := Logger{}
	logger.Warning("", "", nil, "")
	logger.Warning("", "", nil, "")

	var countLog, countWarn int
	LatestLogs.IterateReverse(func(_ string) bool {
		countLog++
		return true
	})
	LatestWarnings.IterateReverse(func(_ string) bool {
		countWarn++
		return true
	})
	// Depending on the test case execution order, the count may be higher if Info test has already run.
	if countLog < 2 || countWarn < 2 {
		t.Fatal(countLog, countWarn)
	}

	logger.Warning("", "", errors.New(""), "")
	logger.Warning("", "", errors.New(""), "")

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
	// Depending on the test case execution order, the count may be higher if Info test has already run.
	if countLog < 4 || countWarn < 4 {
		t.Fatal(countLog, countWarn)
	}
}
