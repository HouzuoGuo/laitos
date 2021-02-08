package passwdrpc

import (
	"testing"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/netboundfileenc"
)

func TestDaemon(t *testing.T) {
	daemon := &Daemon{
		PasswordRegister: netboundfileenc.NewPasswordRegister(10, 10, lalog.DefaultLogger),
	}
	// Initialise with default configuration values
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	if daemon.Address != "0.0.0.0" || daemon.Port != DefaultPort {
		t.Fatalf("wrong default config: actual address is %s, actual port is %d", daemon.Address, daemon.Port)
	}
	// Initialise using custom configuration values
	daemon = &Daemon{
		Address:          "0.0.0.0",
		Port:             TestPort,
		PasswordRegister: netboundfileenc.NewPasswordRegister(10, 10, lalog.DefaultLogger),
	}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Execute rest of the daemon tests
	TestPasswdRPCDaemon(daemon, t)
}
