package feature

import (
	"testing"
)

func TestFeatureSet_SelfTest(t *testing.T) {
	// Initially, no feature other than shell is available from an empty feature set
	features := FeatureSet{}
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(features.LookupByTrigger) != 1 || features.LookupByTrigger[".s"] == nil {
		t.Fatal(features.LookupByTrigger)
	}
	// Configure AES decrypt and see
	features = FeatureSet{AESDecrypt: GetTestAESDecrypt()}
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(features.LookupByTrigger) != 2 {
		t.Fatal(features.LookupByTrigger)
	}
	if errs := features.SelfTest(); len(errs) != 0 {
		t.Fatal(errs)
	}
	// Configure all features via JSON and verify via self test
	features = TestFeatureSet
	features.Initialise()
	if len(features.LookupByTrigger) != 8 {
		t.Skip(features.LookupByTrigger)
	}
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(features.LookupByTrigger) != 8 {
		t.Fatal(features.LookupByTrigger)
	}
	if errs := features.SelfTest(); len(errs) != 0 {
		t.Fatal(errs)
	}
	// Give every feature a configuration error and test again
	features.AESDecrypt.EncryptedFiles["beta"].FilePath = "does not exist"
	features.Facebook.UserAccessToken = "very bad"
	features.SendMail.Mailer.MTAHost = "very bad"
	features.Shell.InterpreterPath = "very bad"
	features.Twilio.AccountSID = "very bad"
	features.Twitter.AccessToken = "very bad"
	features.Twitter.reqSigner.AccessToken = "very bad"
	features.Undocumented1.URL = "very bad"
	features.WolframAlpha.AppID = "very bad"
	errs := features.SelfTest()
	if len(errs) != 8 {
		t.Fatal(len(errs), errs)
	}
}
