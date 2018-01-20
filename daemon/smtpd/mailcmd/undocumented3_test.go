package mailcmd

import (
	"github.com/HouzuoGuo/laitos/inet"
	"testing"
)

func TestUndocumented3_MayReplyTo(t *testing.T) {
	und := Undocumented3{}
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
	und = Undocumented3{URL: "https://github.com", MailAddrSuffix: "@b.c", ToNumber: "b", ReplyEmail: "c"}
	if !und.MayReplyTo(inet.BasicMail{ReplyAddress: "a@b.c"}) {
		t.Fatal("wrong")
	}
}

func TestUndocumented3_SelfTest(t *testing.T) {
	und := Undocumented3{}
	if err := und.SelfTest(); err == nil {
		t.Fatal("did not error")
	}
	und = Undocumented3{URL: "https://github.com", MailAddrSuffix: "@b.c", ToNumber: "b", ReplyEmail: "c"}
	if err := und.SelfTest(); err != nil {
		t.Fatal(err)
	}
	und.URL = "this url does not exist"
	if err := und.SelfTest(); err == nil {
		t.Fatal("did not error")
	}
}

func TestUndocumented3_Execute(t *testing.T) {
	if !TestUndocumented3.IsConfigured() {
		t.Skip()
	}
	if err := TestUndocumented3.SelfTest(); err != nil {
		t.Fatal(err)
	}
	// Nothing to do
	if err := TestUndocumented3.SendMessage("   \r\t\n   "); err == nil {
		t.Fatal("did not error")
	}
	// Do something
	if err := TestUndocumented3.SendMessage("laitos undocumented3 test"); err != nil {
		t.Fatal(err)
	}
}
