package feature

import (
	"testing"
)

func TestFeatureSet_SelfTest(t *testing.T) {
	features := FeatureSet{}
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Apart from shell, none of the features is in a configured state, their tests are skipped automatically.
	if len(features.LookupByTrigger) != 1 {
		t.Fatal(features.LookupByTrigger)
	}
	if errs := features.SelfTest(); len(errs) != 0 {
		t.Fatal(errs)
	}
	// Configure all features via JSON and verify via self test\
	features = TestFeatureSet
	features.Initialise()
	if len(features.LookupByTrigger) != 6 {
		t.Skip()
	}
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(features.LookupByTrigger) != 6 {
		t.Fatal(features.LookupByTrigger)
	}
	if errs := features.SelfTest(); len(errs) != 0 {
		t.Fatal(errs)
	}
	// Give every feature a configuration error and test again
	features.Facebook.UserAccessToken = "very bad"
	features.Twitter.AccessToken = "very bad"
	features.Shell.InterpreterPath = "very bad"
	features.Twilio.AccountSID = "very bad"
	features.Undocumented1.URL = "very bad"
	features.WolframAlpha.AppID = "very bad"
	features.Initialise()
	errs := features.SelfTest()
	if len(errs) != 6 {
		t.Fatal(len(errs), errs)
	}
}
