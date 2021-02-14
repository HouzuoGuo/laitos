package httpproxy

import (
	"testing"

	"github.com/HouzuoGuo/laitos/misc"
)

func TestDaemon(t *testing.T) {
	misc.EnableAWSIntegration = true
	misc.EnablePrometheusIntegration = true
	daemon := &Daemon{}
	// Initialise with default configuration values
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	if daemon.Address != "0.0.0.0" || daemon.Port != DefaultPort || daemon.PerIPLimit != 100 {
		t.Fatalf("wrong default config: actual address is %s, actual port is %d, actual per-ip limit is %d", daemon.Address, daemon.Port, daemon.PerIPLimit)
	}
	// Initialise using custom configuration values
	daemon = &Daemon{
		Address:        "0.0.0.0",
		Port:           TestPort,
		PerIPLimit:     10,
		AllowFromCidrs: []string{"127.0.0.0/8"},
	}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Execute the remainder of daemon tests
	TestHTTPProxyDaemon(daemon, t)
}
