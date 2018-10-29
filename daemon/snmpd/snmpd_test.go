package snmpd

import (
	"github.com/HouzuoGuo/laitos/daemon/snmpd/snmp"
	"strings"
	"testing"
)

func TestSNMPD(t *testing.T) {
	daemon := Daemon{}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "CommunityName") {
		t.Fatalf("%+v %+v\n", err, daemon)
	}
	daemon.CommunityName = "public"
	if err := daemon.Initialise(); err != nil || daemon.Address != "0.0.0.0" || daemon.Port != 161 || daemon.PerIPLimit != len(snmp.OIDSuffixList) {
		t.Fatalf("%+v %+v\n", err, daemon)
	}
	TestServer(&daemon, t)
}
