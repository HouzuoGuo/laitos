package lalog

import (
	"errors"
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
}

func TestLogger_Panicf(t *testing.T) {
	defer func() {
		recover()
	}()
	logger := Logger{}
	logger.Panicf("", "", nil, "")
	t.Fatal("did not panic")
}
