package snmpd

import (
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/daemon/snmpd/snmp"
)

func TestDaemon(t *testing.T) {
	daemon := Daemon{}
	// Initialise with missing parameter
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "CommunityName") {
		t.Fatalf("%+v %+v\n", err, daemon)
	}
	// Initialise with default values
	daemon.CommunityName = "public"
	if err := daemon.Initialise(); err != nil || daemon.Address != "0.0.0.0" || daemon.Port != 161 || daemon.PerIPLimit != len(snmp.OIDSuffixList) {
		t.Fatalf("%+v %+v\n", err, daemon)
	}
	// Avoid binding to default privileged port for this test case
	daemon.Port = 43890
	TestSNMPD(&daemon, t)
}
