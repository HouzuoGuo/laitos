package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/passwdrpc"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/netboundfileenc"
)

func TestReseedPseudoRandAndInBackground(t *testing.T) {
	ReseedPseudoRandAndInBackground()
}

func TestCopyNonEssentialUtilitiesInBackground(t *testing.T) {
	CopyNonEssentialUtilitiesInBackground()
}

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
	done := make(chan struct{}, 0)
	go func() {
		AutoRestart(lalog.Logger{}, "sample", sampleFun)
		done <- struct{}{}
	}()
	start := time.Now()
	// Round 1 quits with an error and it is immediately restarted
	<-restarted
	if time.Now().Sub(start) > 2*time.Second {
		t.Fatal("round 1 took too long")
	}
	// Round 2 quits with an error
	<-restarted
	if time.Now().Sub(start) > 2*time.Second {
		t.Fatal("round 2 took too long")
	}
	// Round 3 is started after another 10 seconds
	<-restarted
	if time.Now().Sub(start) > 12*time.Second {
		t.Fatal("round 3 took too long")
	}
	// Round 4 is started after another 20 seconds
	<-done
	if time.Now().Sub(start) > 32*time.Second {
		t.Fatal("round 4 (successful return) took too long")
	}
}

func TestAutoRestartDuringLockDown(t *testing.T) {
	sampleFun := func() error {
		return errors.New("sample function error")
	}
	done := make(chan struct{}, 0)
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

func TestGetUnlockingPassword(t *testing.T) {
	daemon := &passwdrpc.Daemon{
		Address:          "127.0.0.1",
		Port:             8972,
		PasswordRegister: netboundfileenc.NewPasswordRegister(10, 10, lalog.DefaultLogger),
	}
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Error(err)
			return
		}
	}()
	if !misc.ProbePort(5*time.Second, daemon.Address, daemon.Port) {
		t.Fatal("daemon did not start on time")
	}
	password := getUnlockingPassword(context.Background(), false, *lalog.DefaultLogger, "test-challenge-str", net.JoinHostPort("127.0.0.1", strconv.Itoa(daemon.Port)))
	if password != "" {
		t.Fatal("should not have got a password at this point")
	}
	daemon.PasswordRegister.FulfilIntent("test-challenge-str", "good-password")
	password = getUnlockingPassword(context.Background(), false, *lalog.DefaultLogger, "test-challenge-str", net.JoinHostPort("127.0.0.1", strconv.Itoa(daemon.Port)))
	if password != "good-password" {
		t.Fatalf("did not get the password: %s", password)
	}
}
