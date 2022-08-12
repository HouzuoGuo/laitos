package cli

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

func TestAutoRestart(t *testing.T) {
	// The sample function returns error three times and then returns nothing.
	restarted := make(chan struct{}, 10)
	roundNum := 0
	sampleFun := func() error {
		if roundNum <= 2 {
			restarted <- struct{}{}
			roundNum++
			return fmt.Errorf("round %d", roundNum)
		}
		return nil
	}
	done := make(chan struct{})
	go func() {
		AutoRestart(lalog.Logger{}, "sample", sampleFun)
		done <- struct{}{}
	}()
	start := time.Now()
	// Round 1 quits with an error and it is immediately restarted
	<-restarted
	if time.Since(start) > 2*time.Second {
		t.Fatal("round 1 took too long")
	}
	// Round 2 quits with an error
	<-restarted
	if time.Since(start) > 2*time.Second {
		t.Fatal("round 2 took too long")
	}
	// Round 3 is started after another 10 seconds
	<-restarted
	if time.Since(start) > 12*time.Second {
		t.Fatal("round 3 took too long")
	}
	// Round 4 is started after another 20 seconds
	<-done
	if time.Since(start) > 32*time.Second {
		t.Fatal("round 4 (successful return) took too long")
	}
}

func TestAutoRestartDuringLockDown(t *testing.T) {
	sampleFun := func() error {
		return errors.New("sample function error")
	}
	done := make(chan struct{})
	// While emergency lock down is activated, auto-restart will not perform a restart.
	misc.EmergencyLockDown = true
	defer func() {
		misc.EmergencyLockDown = false
	}()
	go func() {
		AutoRestart(lalog.Logger{}, "sample", sampleFun)
		done <- struct{}{}
	}()
	<-done
}
