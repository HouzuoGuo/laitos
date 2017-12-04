package toolbox

import (
	"github.com/HouzuoGuo/laitos/inet"
	"strings"
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
	conn, err := accountA.ConnectLoginSelect()
	if err != nil {
		t.Fatal(err)
	}
	if num, err := conn.GetNumberMessages(accountA.MailboxName); err != nil || num == 0 {
		t.Fatal(num, err)
	}
	if _, err := conn.GetHeaders(1, 0); err == nil {
		t.Fatal("did not error")
	}
	if _, err := conn.GetHeaders(2, 1); err == nil {
		t.Fatal("did not error")
	}
	// Retrieve headers, make sure it is retrieving three different emails
	headers, err := conn.GetHeaders(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 3 {
		t.Fatal(headers)
	}
	// Retrieve mail body
	msg, err := conn.GetMessage(1)
	if err != nil {
		t.Fatal(err, msg)
	}
	err = inet.WalkMailMessage([]byte(msg), func(prop inet.BasicMail, section []byte) (bool, error) {
		if prop.Subject == "" || prop.ContentType == "" || prop.FromAddress == "" || prop.ReplyAddress == "" {
			t.Fatalf("%+v", prop)
		}
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	conn.LogoutDisconnect()
}

func TestIMAPAccountsPublicServer(t *testing.T) {
	accounts := IMAPAccounts{
		Accounts: map[string]*IMAPS{
			"hotmail": {
				Host:         "imap-mail.outlook.com",
				AuthUsername: "test@test.com",
				AuthPassword: "test",
			},
		},
	}
	if err := accounts.Initialise(); err != nil {
		t.Fatal(err)
	}
	if hotmail := accounts.Accounts["hotmail"]; hotmail.MailboxName != "INBOX" || hotmail.Port != 993 || hotmail.InsecureSkipVerify != false {
		t.Fatalf("%+v", hotmail)
	}
	if err := accounts.SelfTest(); err == nil {
		t.Fatal("did not perform login test")
	}
	if ret := TestIMAPAccounts.Execute(Command{TimeoutSec: 30, Content: MailboxList + "test 1, 2"}); strings.Index(ret.Error.Error(), "find mailbox") == -1 {
		t.Fatal(ret)
	}
	if ret := TestIMAPAccounts.Execute(Command{TimeoutSec: 30, Content: MailboxRead + "test 1"}); strings.Index(ret.Error.Error(), "find mailbox") == -1 {
		t.Fatal(ret)
	}
}

func TestIMAPAccounts_Execute(t *testing.T) {
	if !TestIMAPAccounts.IsConfigured() {
		t.Skip()
	}
	if err := TestIMAPAccounts.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := TestIMAPAccounts.SelfTest(); err != nil {
		t.Fatal(err)
	}
	// Nothing to do
	ret := TestIMAPAccounts.Execute(Command{TimeoutSec: 30, Content: "!@$!@%#%#$@%"})
	if ret.Error != ErrBadMailboxParam {
		t.Fatal(ret)
	}
	// Bad parameters
	if ret := TestIMAPAccounts.Execute(Command{TimeoutSec: 30, Content: MailboxList}); ret.Error != ErrBadMailboxParam {
		t.Fatal(ret)
	}
	if ret := TestIMAPAccounts.Execute(Command{TimeoutSec: 30, Content: MailboxList + "a 1, b"}); ret.Error != ErrBadMailboxParam {
		t.Fatal(ret)
	}
	if ret := TestIMAPAccounts.Execute(Command{TimeoutSec: 30, Content: MailboxRead}); ret.Error != ErrBadMailboxParam {
		t.Fatal(ret)
	}
	if ret := TestIMAPAccounts.Execute(Command{TimeoutSec: 30, Content: MailboxRead + "a b"}); ret.Error != ErrBadMailboxParam {
		t.Fatal(ret)
	}
	if ret := TestIMAPAccounts.Execute(Command{TimeoutSec: 30, Content: MailboxList + "does_not_exist 1, 2"}); strings.Index(ret.Error.Error(), "find mailbox") == -1 {
		t.Fatal(ret)
	}
	if ret := TestIMAPAccounts.Execute(Command{TimeoutSec: 30, Content: MailboxRead + "does_not_exist 1"}); strings.Index(ret.Error.Error(), "find mailbox") == -1 {
		t.Fatal(ret)
	}
	if ret := TestIMAPAccounts.Execute(Command{TimeoutSec: 30, Content: MailboxList + "a 100000000, 100"}); strings.Index(ret.Error.Error(), "skip must be") == -1 {
		t.Fatal(ret)
	}
	// List latest messages
	ret = TestIMAPAccounts.Execute(Command{TimeoutSec: 30, Content: MailboxList + "a 15, 10"})
	t.Log("List", ret.Output)
	if ret.Error != nil || len(ret.Output) < 100 || len(ret.Output) > 2000 {
		t.Fatal(ret)
	}
	// Read one message
	ret2 := TestIMAPAccounts.Execute(Command{TimeoutSec: 30, Content: MailboxRead + "a 5"})
	t.Log("Read", ret2.Output)
	if ret2.Error != nil || len(ret2.Output) < 1 {
		t.Fatal(ret)
	}
}
