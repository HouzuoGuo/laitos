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
	if msg := logger.Format("", "", errors.New("test"), ""); msg != "Error \"test\"" {
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
	if msg := logger.Format("fun", "act", errors.New("test"), strings.Repeat("a", MaxLogMessageLen)); len(msg) != MaxLogMessageLen || !strings.Contains(msg, strings.Repeat("a", 500)) {
		t.Fatal(len(msg), msg)
	}
	if msg := logger.Format("", "", errors.New("test"), ""); msg != `comp[a=1;b=c]: Error "test"` {
		t.Fatal(msg)
	}
}

func TestLogger_Panicf(t *testing.T) {
	defer func() {
		_ = recover()
	}()
	logger := Logger{}
	logger.Panic("", nil, "")
	t.Fatal("did not panic")
}

func TestLogger_Printf(t *testing.T) {
	logger := Logger{
		ComponentName: "test-comp-name",
		ComponentID: []LoggerIDField{
			{
				Key:   "logger-id",
				Value: "logger-id-val",
			},
		},
	}
	logger.Info("adam", nil, "alpha")
	logger.Info("adam", nil, "beta")

	var countLog, countWarn int
	LatestLogs.IterateReverse(func(msg string) bool {
		if strings.Contains(msg, "TestLogger_Printf") {
			countLog++
		}
		return true
	})
	LatestWarnings.IterateReverse(func(msg string) bool {
		if strings.Contains(msg, "TestLogger_Printf") {
			countWarn++
		}
		return true
	})
	if countLog != 2 {
		t.Fatal(countLog, countWarn)
	}

	logger.Info("eve", errors.New(""), "")
	logger.Info("adam", errors.New(""), "")

	countLog = 0
	countWarn = 0
	LatestLogs.IterateReverse(func(msg string) bool {
		if strings.Contains(msg, "TestLogger_Printf") {
			countLog++
		}
		return true
	})
	LatestWarnings.IterateReverse(func(msg string) bool {
		if strings.Contains(msg, "TestLogger_Printf") {
			countWarn++
		}
		return true
	})
	// Depending on the test case execution order, the count may be higher if Warning test has already run.
	if countLog < 4 || countWarn < 2 {
		t.Fatal(countLog, countWarn)
	}

}

func TestLogger_Warningf(t *testing.T) {
	logger := Logger{}
	// The first warning originated from Adam went into in-memory warning buffer
	logger.Warning("adam", nil, "")
	// The second warning is de-duplicated.
	logger.Warning("adam", nil, "")

	var countLog, countWarn int
	LatestLogs.IterateReverse(func(msg string) bool {
		if strings.Contains(msg, "TestLogger_Warningf") {
			countLog++
		}
		return true
	})
	LatestWarnings.IterateReverse(func(msg string) bool {
		if strings.Contains(msg, "TestLogger_Warningf") {
			countWarn++
		}
		return true
	})
	if countLog != 1 || countWarn != 1 {
		t.Fatal(countLog, countWarn)
	}

	// Both warnings will end up in the in-memory warning buffer
	logger.Warning("eve", errors.New(""), "")
	logger.Warning("adam", errors.New(""), "")

	countWarn = 0
	countLog = 0
	LatestLogs.IterateReverse(func(msg string) bool {
		if strings.Contains(msg, "TestLogger_Warningf") {
			countLog++
		}
		return true
	})
	LatestWarnings.IterateReverse(func(msg string) bool {
		if strings.Contains(msg, "TestLogger_Warningf") {
			countWarn++
		}
		return true
	})
	// "adam" and "eve" are recorded in the all-message and warning buffers.
	if countLog != 2 || countWarn != 2 {
		t.Fatal(countLog, countWarn)
	}
}

func TestLogger_MaybeError(t *testing.T) {
	logger := Logger{}
	logger.MaybeMinorError(nil)
	logger.MaybeMinorError(errors.New("testError"))
}

func TestTruncateString(t *testing.T) {
	if s := TruncateString("", -1); s != "" {
		t.Fatal(s)
	}
	if s := TruncateString("", 0); s != "" {
		t.Fatal(s)
	}
	if s := TruncateString("a", 0); s != "" {
		t.Fatal(s)
	}

	if s := TruncateString("aa", 1); s != "a" {
		t.Fatal(s)
	}
	if s := TruncateString("aa", 2); s != "aa" {
		t.Fatal(s)
	}
	if s := TruncateString("aa", 3); s != "aa" {
		t.Fatal(s)
	}

	if s := TruncateString("01234567890123456789", 10); s != "0123456789" {
		t.Fatal(s)
	}
	if s := TruncateString("01234567890123456789", 17); s != "01234567890123456" {
		t.Fatal(s)
	}
	if s := TruncateString("01234567890123456789", 18); s != "0...(truncated)..." {
		t.Fatal(s)
	}
	if s := TruncateString("01234567890123456789", 19); s != "0...(truncated)...9" {
		t.Fatal(s)
	}
	if s := TruncateString("012345678901234567890123456789", 25); s != "0123...(truncated)...6789" {
		t.Fatal(s)
	}

	if s := TruncateString(strings.Repeat("a", 1000), 500); !strings.Contains(s, strings.Repeat("a", 241)) {
		t.Fatal(s)
	}
}

func TestLintString(t *testing.T) {
	if s := LintString("", -1); s != "" {
		t.Fatal(s)
	}
	if s := LintString("", 0); s != "" {
		t.Fatal(s)
	}
	if s := LintString("abc", 1); s != "a" {
		t.Fatal(s)
	}

	a := LintString("\x01\x08 a \x0e\x1f b\n \x7f c\t \x80", 100)
	match := "__ a __ b\n _ c\t _"
	if a != match {
		t.Fatalf("\n%s\n%s\n%v\n%v\n", a, match, []byte(a), []byte(match))
	}
}
