package feature

import "testing"

func TestUndocumented1_Execute(t *testing.T) {
	if !TestUndocumented1.IsConfigured() {
		t.Skip()
	}
	if err := TestUndocumented1.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := TestUndocumented1.SelfTest(); err != nil {
		t.Fatal(err)
	}
	// Nothing to do
	if ret := TestUndocumented1.Execute(Command{TimeoutSec: 30, Content: "   \r\t\n   "}); ret.Error == nil {
		t.Fatal("did not error")
	}
	// Do something
	if ret := TestUndocumented1.Execute(Command{TimeoutSec: 30, Content: "testtest123"}); ret.Error != nil {
		t.Fatal(ret.Error)
	}
}
