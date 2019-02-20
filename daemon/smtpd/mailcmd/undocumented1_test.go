package mailcmd

import (
	"testing"

	"github.com/HouzuoGuo/laitos/inet"
)

func TestUndocumented1_MayReplyTo(t *testing.T) {
	und := Undocumented1{}
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
	und = Undocumented1{URL: "https://github.com", MailAddrSuffix: "@b.c", ReplyAddress: "b", MessageID: "c", GUID: "d"}
	if !und.MayReplyTo(inet.BasicMail{ReplyAddress: "a@b.c"}) {
		t.Fatal("wrong")
	}
}

func TestUndocumented1_SelfTest(t *testing.T) {
	und := Undocumented1{}
	if err := und.SelfTest(); err == nil {
		t.Fatal("did not error")
	}
	und = Undocumented1{URL: "https://github.com", MailAddrSuffix: "a", ReplyAddress: "b", MessageID: "c", GUID: "d"}
	if err := und.SelfTest(); err != nil {
		t.Fatal(err)
	}
	und.URL = "this url does not exist"
	if err := und.SelfTest(); err == nil {
		t.Fatal("did not error")
	}
}

func TestUndocumented1_Execute(t *testing.T) {
	if !TestUndocumented1.IsConfigured() {
		t.Log("skip because TestUndocumented1 is not configured")
		return
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
