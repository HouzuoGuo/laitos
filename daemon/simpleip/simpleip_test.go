package simpleip

import "testing"

func TestSimpleIPDaemon(t *testing.T) {
	daemon := &Daemon{}
	// Empty configuration is also valid
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}

	daemon = &Daemon{ActiveUserNames: "howard (houzuo) guo", QOTD: "hello from howard"}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	TestDaemon(daemon, t)
}
