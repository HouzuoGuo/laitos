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
	sampleRound := 0
	sampleFun := func() error {
		if sampleRound <= 2 {
			sampleRound++
			return fmt.Errorf("round %d", sampleRound)
		}
		return nil
	}
	var returnedFromRestart bool
	go func() {
		AutoRestart(lalog.Logger{}, "sample", sampleFun)
		returnedFromRestart = true
	}()
	// Round 0 quits with an error and it is immediately restarted
	time.Sleep(1 * time.Second)
	// Round 1 quits with an error
	if sampleRound != 2 {
		t.Fatal(sampleRound)
	}
	// Round 2 is started after 10 seconds
	time.Sleep(10 * time.Second)
	// Round 2 quits with an error
	if sampleRound != 3 {
		t.Fatal(sampleRound)
	}
	// Round 3 is started after 20 seconds
	time.Sleep(20 * time.Second)
	if sampleRound != 3 {
		t.Fatal(sampleRound)
	}
	// Round 3 quits successfuly, no further restart is required.
	if !returnedFromRestart {
		t.Fatal("did not return")
	}
}

func TestAutoRestartDuringLockDown(t *testing.T) {
	sampleFun := func() error {
		return errors.New("sample function error")
	}
	var returnedFromRestart bool
	go func() {
		AutoRestart(lalog.Logger{}, "sample", sampleFun)
		returnedFromRestart = true
	}()
	// Turn on emergency lock-down
	misc.EmergencyLockDown = true
	defer func() {
		misc.EmergencyLockDown = false
	}()
	// AutoRestart keeps the sample function ablive, but it shall quit after emergency lock-down.
	// Wait past the second restart
	time.Sleep(15 * time.Second)
	if !returnedFromRestart {
		t.Fatal("did not return")
	}
}

func TestGetUnlockingPassword(t *testing.T) {
	daemon := &passwdrpc.Daemon{
		Address:          "0.0.0.0",
		Port:             8972,
		PasswordRegister: netboundfileenc.NewPasswordRegister(10, 10, lalog.DefaultLogger),
	}
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Error(err)
			return
		}
	}()
	if !misc.ProbePort(1*time.Second, "127.0.0.1", daemon.Port) {
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
