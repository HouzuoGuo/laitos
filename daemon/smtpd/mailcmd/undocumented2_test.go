package mailcmd

import (
	"github.com/HouzuoGuo/laitos/inet"
	"testing"
)

func TestUndocumented2_MayReplyTo(t *testing.T) {
	und := Undocumented2{}
	if und.MayReplyTo(inet.BasicMail{}) {
		t.Fatal("wrong")
	}
	if und.MayReplyTo(inet.BasicMail{ReplyAddress: "a@b.c"}) {
		t.Fatal("wrong")
	}
	und.MailAddrSuffix = "@b.c"
	if und.MayReplyTo(inet.BasicMail{ReplyAddress: "a@b.c"}) {
		t.Fatal("wrong")
	}
	und = Undocumented2{URL: "https://github.com", MailAddrSuffix: "@b.c", MsisDN: "b", From: "c"}
	if !und.MayReplyTo(inet.BasicMail{ReplyAddress: "a@b.c"}) {
		t.Fatal("wrong")
	}
}

func TestUndocumented2_SelfTest(t *testing.T) {
	und := Undocumented2{}
	if err := und.SelfTest(); err == nil {
		t.Fatal("did not error")
	}
	und = Undocumented2{URL: "https://github.com", MailAddrSuffix: "@b.c", MsisDN: "b", From: "c"}
	if err := und.SelfTest(); err != nil {
		t.Fatal(err)
	}
	und.URL = "this url does not exist"
	if err := und.SelfTest(); err == nil {
		t.Fatal("did not error")
	}
}

func TestUndocumented2_Execute(t *testing.T) {
	if !TestUndocumented2.IsConfigured() {
		t.Skip()
	}
	if err := TestUndocumented2.SelfTest(); err != nil {
		t.Fatal(err)
	}
	// Nothing to do
	if err := TestUndocumented2.SendMessage("   \r\t\n   "); err == nil {
		t.Fatal("did not error")
	}
	// Do something
	if err := TestUndocumented2.SendMessage("laitos undocumented2 test"); err != nil {
		t.Fatal(err)
	}
}
