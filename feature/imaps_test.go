package feature

import (
	"github.com/HouzuoGuo/laitos/email"
	"testing"
)

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
	// IMAPS account test
	accountA := TestIMAPAccounts.Accounts["a"]
	if err := accountA.ConnectLoginSelect(); err != nil {
		t.Fatal(err)
	}
	if num, err := accountA.GetNumberMessages(); err != nil || num == 0 {
		t.Fatal(num, err)
	}
	if _, err := accountA.GetHeaders(1, 0); err == nil {
		t.Fatal("did not error")
	}
	if _, err := accountA.GetHeaders(2, 1); err == nil {
		t.Fatal("did not error")
	}
	// Retrieve headers, make sure it is retrieving three different emails
	headers, err := accountA.GetHeaders(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 3 {
		t.Fatal(headers)
	}
	// Retrieve mail body
	msg, err := accountA.GetMessage(1)
	if err != nil {
		t.Fatal(err, msg)
	}
	err = email.WalkMessage([]byte(msg), func(prop email.BasicProperties, section []byte) (bool, error) {
		if prop.Subject == "" || prop.ContentType == "" || prop.FromAddress == "" || prop.ReplyAddress == "" {
			t.Fatalf("%+v", prop)
		}
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	accountA.DisconnectLogout()
}

func TestIMAPAccounts_Execute(t *testing.T) {

}
