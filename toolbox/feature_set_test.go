package toolbox

import (
	"fmt"
	"reflect"
	"testing"
)

func TestFeatureSet_SelfTest(t *testing.T) {
	// Initially, an empty FeatureSet should have three features pre-enabled - shell, environment control, and public contacts.
	features := FeatureSet{}
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(features.LookupByTrigger) != 3 ||
		features.LookupByTrigger[".c"] == nil ||
		features.LookupByTrigger[".e"] == nil ||
		features.LookupByTrigger[".s"] == nil {
		t.Fatal(features.LookupByTrigger)
	}
	// Configure AES decrypt and 2fa code generator
	features = FeatureSet{AESDecrypt: GetTestAESDecrypt(), TwoFACodeGenerator: GetTestTwoFACodeGenerator()}
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Public contacts, environment control, shell commands, AESDecrypt, TwoFACodeGenerator
	if len(features.LookupByTrigger) != 5 {
		t.Fatal(features.LookupByTrigger)
	}
	if errs := features.SelfTest(); len(errs) != 0 {
		t.Fatal(errs)
	}
	if triggers := features.GetTriggers(); !reflect.DeepEqual(triggers, []string{".2", ".a", ".c", ".e", ".s"}) {
		t.Fatal(triggers)
	}
	// Configure all features via JSON and verify via self test
	features = TestFeatureSet
	features.Initialise()
	if len(features.LookupByTrigger) != 12 {
		t.Skip(features.LookupByTrigger)
	}
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(features.LookupByTrigger) != 12 {
		t.Fatal(features.LookupByTrigger)
	}
	if errs := features.SelfTest(); len(errs) != 0 {
		t.Fatal(errs)
	}
	// Give nearly every feature a configuration error and test again
	features.AESDecrypt.EncryptedFiles["beta"].FilePath = "does not exist"
	features.Facebook.UserAccessToken = "very bad"
	features.IMAPAccounts.Accounts = map[string]*IMAPS{
		"a": {
			Host:         "does-not-exist",
			Port:         1234,
			MailboxName:  "does-not-exist",
			AuthUsername: "does-not-exist",
			AuthPassword: "does-not-exist",
		},
	}
	features.IMAPAccounts.Initialise()
	features.SendMail.MailClient.MTAHost = "very bad"
	features.Shell.InterpreterPath = "very bad"
	features.Twilio.AccountSID = "very bad"
	features.Twitter.AccessToken = "very bad"
	features.Twitter.reqSigner.AccessToken = "very bad"
	features.TwoFACodeGenerator.SecretFile.FilePath = "does not exist"
	features.WolframAlpha.AppID = "very bad"
	errs := features.SelfTest()
	if len(errs) != 9 {
		for prefix, err := range errs {
			fmt.Printf("Error from %s: %v\n\n", prefix, err)
		}
		t.Fatal("wrong number of errors", len(errs))
	}
}
