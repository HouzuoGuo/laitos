package feature

import "testing"

func TestSendMail_Execute(t *testing.T) {
	if !TestSendMail.IsConfigured() {
		t.Skip()
	}
	if err := TestSendMail.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := TestSendMail.SelfTest(); err != nil {
		t.Fatal(err)
	}
	if ret := TestSendMail.Execute(Command{TimeoutSec: 10, Content: "wrong"}); ret.Error != ErrBadSendMailParam {
		t.Fatal(ret)
	}
	if ret := TestSendMail.Execute(Command{TimeoutSec: 10, Content: `guohouzuo@gmail.com "laitos send mail test" this is laitos send mail test`}); ret.Error != nil || ret.Output != "29" {
		t.Fatal(ret)
	}
}
