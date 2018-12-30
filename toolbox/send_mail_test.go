package toolbox

import (
	"testing"
)

func TestSendMail_Execute(t *testing.T) {
	if !TestSendMail.IsConfigured() {
		t.Skip("not configured")
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

	/*
		Make sure you have altered SARContacts to non-functional addresses before conducting this test...
		if ret := TestSendMail.Execute(Command{TimeoutSec: 10, Content: `SOS@sos "this is a test pls ignore" this is a test pls ignore`}); ret.Error != nil || ret.Output != "Sending SOS" {
			t.Fatal(ret)
		}
		time.Sleep(10 * time.Second)
	*/
}
