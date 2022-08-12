package cli

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/passwdrpc"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/netboundfileenc"
)

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
	if !misc.ProbePort(30*time.Second, daemon.Address, daemon.Port) {
		t.Fatal("daemon did not start on time")
	}
	password := GetUnlockingPassword(context.Background(), false, *lalog.DefaultLogger, "test-challenge-str", net.JoinHostPort("127.0.0.1", strconv.Itoa(daemon.Port)))
	if password != "" {
		t.Fatal("should not have got a password at this point")
	}
	daemon.PasswordRegister.FulfilIntent("test-challenge-str", "good-password")
	password = GetUnlockingPassword(context.Background(), false, *lalog.DefaultLogger, "test-challenge-str", net.JoinHostPort("127.0.0.1", strconv.Itoa(daemon.Port)))
	if password != "good-password" {
		t.Fatalf("did not get the password: %s", password)
	}
}
