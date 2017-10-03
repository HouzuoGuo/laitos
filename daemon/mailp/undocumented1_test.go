package mailp

import (
	"github.com/HouzuoGuo/laitos/inet"
	"testing"
)

func TestUndocumented1_MayReplyTo(t *testing.T) {
	und := Undocumented1{}
	if und.MayReplyTo(inet.BasicProperties{}) {
		t.Fatal("wrong")
	}
	if und.MayReplyTo(inet.BasicProperties{ReplyAddress: "a@b.c"}) {
		t.Fatal("wrong")
	}
	und.MailAddrSuffix = "@b.c"
	if und.MayReplyTo(inet.BasicProperties{ReplyAddress: "a@b.c"}) {
		t.Fatal("wrong")
	}
	und = Undocumented1{URL: "https://github.com", MailAddrSuffix: "@b.c", ReplyAddress: "b", MessageID: "c", GUID: "d"}
	if !und.MayReplyTo(inet.BasicProperties{ReplyAddress: "a@b.c"}) {
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
