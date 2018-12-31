package simpleipsvcd

import "testing"

func TestSimpleIPDaemon(t *testing.T) {
	daemon := &Daemon{}
	// Empty configuration is also valid
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	if daemon.ActiveUsersPort != 11 || daemon.DayTimePort != 12+1 || daemon.QOTDPort != 17 {
		t.Fatal(daemon)
	}

	daemon = &Daemon{
		ActiveUserNames: "howard (houzuo) guo",
		QOTD:            "hello from howard",
		ActiveUsersPort: 10120,
		DayTimePort:     18949,
		QOTDPort:        64642,
	}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	TestSimpleIPSvcD(daemon, t)
}
