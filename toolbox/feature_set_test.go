package toolbox

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// StringContainsAllOf returns an error with detailed message if the input string does not contain all of the substring markers.
func StringContainsAllOf(s string, markers []Trigger) error {
	for _, marker := range markers {
		if !strings.Contains(s, string(marker)) {
			return fmt.Errorf("string did not contain marker \"%s\"", marker)
		}
	}
	return nil
}

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
	if err := features.SelfTest(); err != nil {
		t.Fatal(err)
	}
	if triggers := features.GetTriggers(); !reflect.DeepEqual(triggers, []string{".2", ".a", ".c", ".e", ".s"}) {
		t.Fatal(triggers)
	}
	// Configure all features via JSON and verify via self test
	features = TestFeatureSet
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := features.SelfTest(); err != nil {
		t.Fatal(err)
	}
	if len(features.LookupByTrigger) != 12 {
		t.Skip(features.LookupByTrigger)
	}
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(features.LookupByTrigger) != 12 {
		t.Fatal(features.LookupByTrigger)
	}
	if err := features.SelfTest(); err != nil {
		t.Fatal(err)
	}
	/*
		Give nearly every feature a configuration error and expect them to be reported in self test.
		Usually, a configuration change must be followed by reinitialisation, however here I am taking a shortcut by
		directly manipulating the internal feature state, especially in the case of Twitter AccessToken.
	*/
	features.AESDecrypt.EncryptedFiles[TestAESDecryptFileBetaName].FilePath = "does not exist"
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

	errString := features.SelfTest().Error()

	findAllErr := StringContainsAllOf(errString, []Trigger{
		features.AESDecrypt.Trigger(),
		features.Facebook.Trigger(),
		features.IMAPAccounts.Trigger(),
		features.SendMail.Trigger(),
		features.Shell.Trigger(),
		features.Twilio.Trigger(),
		features.Twitter.Trigger(),
		features.TwoFACodeGenerator.Trigger(),
		features.WolframAlpha.Trigger(),
	})

	if findAllErr != nil {
		t.Fatal(findAllErr)
	}
}
