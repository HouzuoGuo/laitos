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
		Address:         "127.0.0.1",
		ActiveUserNames: "howard (houzuo) guo",
		QOTD:            "hello from howard",
		ActiveUsersPort: 15236,
		DayTimePort:     11673,
		QOTDPort:        31678,
	}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	TestSimpleIPSvcD(daemon, t)
}
