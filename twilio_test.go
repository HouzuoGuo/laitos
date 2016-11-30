package main

import "testing"

func TestCmdRunTwilio(t *testing.T) {
	sh := CommandRunner{TimeoutSec: 1, TruncateLen: 30, Twilio: TwilioClient{PhoneNumber: "1", AccountSID: "a", AuthSecret: "b"}}
	if out := sh.RunCommand("#c+49123456789 hi there", false); out == twilioParamError {
		t.Fatal("should not fail this way")
	} else if out := sh.RunCommand("#t+49123456789 hi there", false); out == twilioParamError {
		t.Fatal("should not fail this way")
	}
	if out := sh.RunCommand("#c +49123456789 hi there", false); out == twilioParamError {
		t.Fatal("should not fail this way")
	} else if out := sh.RunCommand("#t +49123456789 hi there", false); out == twilioParamError {
		t.Fatal("should not fail this way")
	}
	if out := sh.RunCommand("#c hi +49123456789 there", false); out == twilioParamError {
		t.Fatal("should not fail this way")
	} else if out := sh.RunCommand("#t hi +49123456789 there", false); out == twilioParamError {
		t.Fatal("should not fail this way")
	}
	if out := sh.RunCommand("#chi there", false); out != twilioParamError {
		t.Fatal("did not fail expectedly")
	} else if out := sh.RunCommand("#thi there", false); out != twilioParamError {
		t.Fatal("did not fail expectedly")
	}
	if out := sh.RunCommand("#c2347891hi there", false); out != twilioParamError {
		t.Fatal("did not fail expectedly")
	} else if out := sh.RunCommand("#t147897hi there", false); out != twilioParamError {
		t.Fatal("did not fail expectedly")
	}
}
