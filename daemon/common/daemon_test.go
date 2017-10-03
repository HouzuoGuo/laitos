package common

import (
	"errors"
	"testing"
	"time"
)

var crashingDaemonStartAttempts int

type CrashingDaemon struct {
}

func (c *CrashingDaemon) StartAndBlock() error {
	crashingDaemonStartAttempts++
	panic("crash")
}
func (c *CrashingDaemon) Stop() {
}

var cannotStartDaemonStartAttempts int

type CannotStartDaemon struct {
}

func (c *CannotStartDaemon) StartAndBlock() error {
	cannotStartDaemonStartAttempts++
	return errors.New("will not start")
}
func (c *CannotStartDaemon) Stop() {
}

var wellBehavedDaemonStartAttempts int

type WellBehavedDaemon struct {
}

func (c *WellBehavedDaemon) StartAndBlock() error {
	wellBehavedDaemonStartAttempts++
	for {
		time.Sleep(1 * time.Hour)
	}
}
func (c *WellBehavedDaemon) Stop() {
}

func TestSupervisor(t *testing.T) {
	sCrash := NewSupervisor(new(CrashingDaemon), 1, "CrashingDaemon")
	go func() {
		if err := sCrash.Start(); err != nil {
			t.Fatal("Unexpected error return", err)
		}
	}()
	time.Sleep(3 * time.Second)
	// Every second, a new start attempt should be made.
	if crashingDaemonStartAttempts < 2 || crashingDaemonStartAttempts > 4 {
		t.Fatal(crashingDaemonStartAttempts)
	}

	sError := NewSupervisor(new(CannotStartDaemon), 1, "CannotStartDaemon")
	if err := sError.Start(); err == nil || err.Error() != "will not start" {
		t.Fatal("did not error", err)
	}
	// Only one startup attempt should be made for a daemon that won't start
	if cannotStartDaemonStartAttempts != 1 {
		t.Fatal(cannotStartDaemonStartAttempts)
	}

	sGood := NewSupervisor(new(WellBehavedDaemon), 1, "WellBehavedDaemon")
	go func() {
		if err := sGood.Start(); err != nil {
			t.Fatal("Unexpected error return", err)
		}
	}()
	time.Sleep(3 * time.Second)
	// Only one attempt should be made for a daemon that starts
	if wellBehavedDaemonStartAttempts != 1 {
		t.Fatal(wellBehavedDaemonStartAttempts)
	}
}
