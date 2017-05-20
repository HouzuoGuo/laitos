package mailp

import "testing"

func TestUndocumented1_Execute(t *testing.T) {
	if !TestUndocumented1.IsConfigured() {
		t.Skip()
	}
	if err := TestUndocumented1.SelfTest(); err != nil {
		t.Fatal(err)
	}
	// Nothing to do
	if err := TestUndocumented1.SendMessage("   \r\t\n   "); err == nil {
		t.Fatal("did not error")
	}
	// Do something
	if err := TestUndocumented1.SendMessage("laitos undocumented1 test"); err != nil {
		t.Fatal(err)
	}
}
