package feature

import (
	"strconv"
	"testing"
)

func TestTwilio_Execute(t *testing.T) {
	if !TestTwilio.IsConfigured() {
		t.Skip()
	}
	if err := TestTwilio.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := TestTwilio.SelfTest(); err != nil {
		t.Fatal(err)
	}
	// Nothing to do
	if ret := TestTwilio.Execute(Command{TimeoutSec: 30, Content: "!@$!@%#%#$@%"}); ret.Error == nil {
		t.Fatal("did not error")
	}
	// Sending an empty SMS should result in error
	if ret := TestTwilio.Execute(Command{TimeoutSec: 30, Content: TwilioSendSMS + "+123456"}); ret.Error == nil {
		t.Fatal("did not error")
	}
	// Send an SMS
	message := "test pls ignore"
	expectedOutput := strconv.Itoa(len(TestTwilio.TestPhoneNumber) + len(message))
	if ret := TestTwilio.Execute(Command{TimeoutSec: 30, Content: TwilioSendSMS + TestTwilio.TestPhoneNumber + "," + message}); ret.Error != nil || ret.Output != expectedOutput {
		t.Fatal(ret)
	}
	// Making a call without a message should result in error
	if ret := TestTwilio.Execute(Command{TimeoutSec: 30, Content: TwilioMakeCall + "+123456"}); ret.Error == nil {
		t.Fatal("did not error")
	}
	// Make a call
	if ret := TestTwilio.Execute(Command{TimeoutSec: 30, Content: TwilioMakeCall + TestTwilio.TestPhoneNumber + "," + message}); ret.Error != nil || ret.Output != expectedOutput {
		t.Fatal(ret)
	}
}
