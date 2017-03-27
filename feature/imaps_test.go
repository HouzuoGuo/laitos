package feature

import "testing"

func TestIMAPS(t *testing.T) {
	if !TestIMAPAccounts.IsConfigured() {
		t.Skip()
	}
	if err := TestIMAPAccounts.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := TestIMAPAccounts.SelfTest(); err != nil {
		t.Fatal(err)
	}
}