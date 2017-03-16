package named

import (
	"strings"
	"testing"
)

func TestDNSD_StartAndBlock(t *testing.T) {
	t.Skip()
	daemon := DNSD{}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen address") == -1 {
		t.Fatal(err)
	}
	daemon.ListenAddress = "0.0.0.0"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen port") == -1 {
		t.Fatal(err)
	}
	daemon.ListenPort = 53535
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "ForwardTo") == -1 {
		t.Fatal(err)
	}
	daemon.ForwardTo = "128.8.8.8"
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "PerIPLimit") == -1 {
		t.Fatal(err)
	}
	daemon.PerIPLimit = 1000
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "allowable IP") == -1 {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = []string{"127", ""}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "all allowable IP prefixes") == -1 {
		t.Fatal(err)
	}
	daemon.AllowQueryIPPrefixes = []string{"127"}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := daemon.StartAndBlock(); err != nil {
		t.Fatal(err)
	}

	// Wireshark is quite helpful in showing the hex string
	//githubComQuery, err := hex.DecodeString("00163e356685989096e3a8e908004500003890e900004011ccb90aa0057b0aa002585ada003500241d48bdb7010000010000000000000667697468756203636f6d0000010001")
	//net.Dial
}
