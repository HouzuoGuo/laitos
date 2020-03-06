package toolbox

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
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
	// Preparation copies PhantomJS executable into a utilities directory and adds it to program $PATH.
	misc.PrepareUtilities(lalog.Logger{})
	// Initially, an empty FeatureSet should have four features pre-enabled - shell, environment control, public contacts, RSS.
	features := FeatureSet{}
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	if len(features.LookupByTrigger) != 6 ||
		features.LookupByTrigger[".0m"] == nil || // store&forward command processor
		features.LookupByTrigger[".c"] == nil || // public contacts
		features.LookupByTrigger[".e"] == nil || // environment control
		features.LookupByTrigger[".j"] == nil || // joke
		features.LookupByTrigger[".r"] == nil || // RSS reader
		features.LookupByTrigger[".s"] == nil { // shell
		t.Fatal(features.LookupByTrigger)
	}
	// Configure AES decrypt and 2fa code generator
	features = FeatureSet{AESDecrypt: GetTestAESDecrypt(), TwoFACodeGenerator: GetTestTwoFACodeGenerator()}
	if err := features.Initialise(); err != nil {
		t.Fatal(err)
	}
	// 6 always-available features + 2 newly configured features (AES + 2FA)
	if len(features.LookupByTrigger) != 8 {
		t.Fatal(features.LookupByTrigger)
	}
	if err := features.SelfTest(); err != nil {
		t.Fatal(err)
	}
	if triggers := features.GetTriggers(); !reflect.DeepEqual(triggers, []string{".0m", ".2", ".a", ".c", ".e", ".j", ".r", ".s"}) {
		t.Fatal(triggers)
	}

	// Configure all features via JSON and verify via self test
	/*
		features = TestFeatureSet
		if err := features.Initialise(); err != nil {
			t.Fatal(err)
		}
		if err := features.SelfTest(); err != nil {
			t.Fatal(err)
		}
		if len(features.LookupByTrigger) != 15 {
			t.Skip(features.LookupByTrigger)
		}
		if err := features.Initialise(); err != nil {
			t.Fatal(err)
		}
		if len(features.LookupByTrigger) != 15 {
			t.Fatal(features.LookupByTrigger)
		}
		if err := features.SelfTest(); err != nil {
			t.Fatal(err)
		}
	*/

	/*
		Give nearly every feature a configuration error and expect them to be reported in self test.
		Usually, a configuration change must be followed by reinitialisation, however here I am taking a shortcut by
		directly manipulating the internal feature state, especially in the case of Twitter AccessToken.
	*/
	features.AESDecrypt.EncryptedFiles[TestAESDecryptFileBetaName].FilePath = "does not exist"
	features.IMAPAccounts.Accounts = map[string]*IMAPS{
		"a": {
			Host:         "does-not-exist",
			Port:         1234,
			MailboxName:  "does-not-exist",
			AuthUsername: "does-not-exist",
			AuthPassword: "does-not-exist",
		},
	}
	if err := features.IMAPAccounts.Initialise(); err != nil {
		t.Fatal(err)
	}
	features.RSS.Sources[0] = "this rss url does not work"
	features.SendMail.MailClient = inet.MailClient{
		MailFrom:     "very bad",
		MTAHost:      "very bad",
		MTAPort:      123,
		AuthUsername: "very bad",
		AuthPassword: "very bad",
	}
	features.Shell.InterpreterPath = "very bad"
	features.TextSearch.FilePaths = map[string]string{"file": "does notexist"}
	features.Twilio = Twilio{
		PhoneNumber:     "very bad",
		AccountSID:      "very bad",
		AuthToken:       "very bad",
		TestPhoneNumber: "very bad",
	}
	features.Twitter.AccessToken = "very bad"
	features.Twitter.reqSigner = &inet.OAuthHeader{AccessToken: "very bad"}
	features.TwoFACodeGenerator.SecretFile.FilePath = "does not exist"
	features.WolframAlpha.AppID = "very bad"

	fmt.Println("initialisation error: ", features.Initialise())

	errString := features.SelfTest().Error()

	findAllErr := StringContainsAllOf(errString, []Trigger{
		features.AESDecrypt.Trigger(),
		features.IMAPAccounts.Trigger(),
		features.RSS.Trigger(),
		features.SendMail.Trigger(),
		features.Shell.Trigger(),
		features.TextSearch.Trigger(),
		features.Twilio.Trigger(),
		features.Twitter.Trigger(),
		features.TwoFACodeGenerator.Trigger(),
		features.WolframAlpha.Trigger(),
	})

	if findAllErr != nil {
		t.Fatal(findAllErr)
	}
}
