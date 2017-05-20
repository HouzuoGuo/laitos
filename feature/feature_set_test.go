package feature

import (
	"reflect"
	"testing"
)

func TestFeatureSet_SelfTest(t *testing.T) {
	// Initially, no feature other than shell and EnvControl are available from an empty feature set
	features := FeatureSet{}
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(features.LookupByTrigger) != 2 || features.LookupByTrigger[".s"] == nil || features.LookupByTrigger[".e"] == nil {
		t.Fatal(features.LookupByTrigger)
	}
	// Configure AES decrypt and see
	features = FeatureSet{AESDecrypt: GetTestAESDecrypt()}
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(features.LookupByTrigger) != 3 {
		t.Fatal(features.LookupByTrigger)
	}
	if errs := features.SelfTest(); len(errs) != 0 {
		t.Fatal(errs)
	}
	// Get triggers of configured features
	if triggers := features.GetTriggers(); !reflect.DeepEqual(triggers, []string{".a", ".e", ".s"}) {
		t.Fatal(triggers)
	}
	// Configure all features via JSON and verify via self test
	features = TestFeatureSet
	features.Initialise()
	if len(features.LookupByTrigger) != 9 {
		t.Skip(features.LookupByTrigger)
	}
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(features.LookupByTrigger) != 9 {
		t.Fatal(features.LookupByTrigger)
	}
	if errs := features.SelfTest(); len(errs) != 0 {
		t.Fatal(errs)
	}
	// Give every feature a configuration error and test again
	features.AESDecrypt.EncryptedFiles["beta"].FilePath = "does not exist"
	features.Facebook.UserAccessToken = "very bad"
	features.IMAPAccounts.Accounts = nil
	features.SendMail.Mailer.MTAHost = "very bad"
	features.Shell.InterpreterPath = "very bad"
	features.Twilio.AccountSID = "very bad"
	features.Twitter.AccessToken = "very bad"
	features.Twitter.reqSigner.AccessToken = "very bad"
	features.WolframAlpha.AppID = "very bad"
	errs := features.SelfTest()
	// There is no way to trigger a fault in env_info, hence there should be 8 failures instead of 9.
	if len(errs) != 8 {
		t.Fatal(len(errs), errs)
	}
}
