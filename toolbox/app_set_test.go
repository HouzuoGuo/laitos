package toolbox

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/inet"
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

func TestFeatureSet_InitSelfTest(t *testing.T) {
	// Several apps will work without explicit configuration
	apps := FeatureSet{}
	if err := apps.Initialise(); err != nil {
		t.Fatal(err)
	}
	enabledByDefaultApps := []Trigger{
		(&EnvControl{}).Trigger(),
		(&Joke{}).Trigger(),
		(&MessageBank{}).Trigger(),
		(&MessageProcessor{}).Trigger(),
		(&NetBoundFileEncryption{}).Trigger(),
		(&PublicContact{}).Trigger(),
		(&RSS{}).Trigger(),
		(&Shell{}).Trigger(),
	}
	if len(apps.LookupByTrigger) != len(enabledByDefaultApps) {
		t.Fatal(apps.LookupByTrigger)
	}
	for _, triggerStr := range enabledByDefaultApps {
		if _, exists := apps.LookupByTrigger[triggerStr]; !exists {
			t.Fatalf("app %s is missing", triggerStr)
		}
	}
	// Validate self-test result from AES encrypted text search and 2FA code generator in addition to the apps above
	apps = FeatureSet{AESDecrypt: GetTestAESDecrypt(), TwoFACodeGenerator: GetTestTwoFACodeGenerator()}
	if err := apps.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Always-available apps + 2 newly configured features (AES + 2FA)
	if len(apps.LookupByTrigger) != len(enabledByDefaultApps)+2 {
		t.Fatal(apps.LookupByTrigger)
	}
	if err := apps.SelfTest(); err != nil {
		t.Fatal(err)
	}
	if triggers := apps.GetTriggers(); !reflect.DeepEqual(triggers, []string{".0m", ".2", ".a", ".b", ".c", ".e", ".j", ".nbe", ".r", ".s"}) {
		t.Fatal(triggers)
	}
}

func TestFeatureSet_InitSelfTestErr(t *testing.T) {
	// Configure AES encrypted text search and 2FA in addition to the always-available apps
	apps := FeatureSet{AESDecrypt: GetTestAESDecrypt(), TwoFACodeGenerator: GetTestTwoFACodeGenerator()}
	if err := apps.Initialise(); err != nil {
		t.Fatal(err)
	}
	apps.AESDecrypt.EncryptedFiles[TestAESDecryptFileBetaName].FilePath = "does not exist"
	apps.IMAPAccounts.Accounts = map[string]*IMAPS{
		"a": {
			Host:         "does-not-exist",
			Port:         1234,
			MailboxName:  "does-not-exist",
			AuthUsername: "does-not-exist",
			AuthPassword: "does-not-exist",
		},
	}
	if err := apps.IMAPAccounts.Initialise(); err != nil {
		t.Fatal(err)
	}
	apps.RSS.Sources[0] = "this rss url does not work"
	apps.SendMail.MailClient = inet.MailClient{
		MailFrom:     "very bad",
		MTAHost:      "very bad",
		MTAPort:      123,
		AuthUsername: "very bad",
		AuthPassword: "very bad",
	}
	apps.Shell.InterpreterPath = "very bad"
	apps.TextSearch.FilePaths = map[string]string{"file": "does notexist"}
	apps.Twilio = Twilio{
		PhoneNumber: "very bad",
		AccountSID:  "very bad",
		AuthToken:   "very bad",
	}
	apps.Twitter = Twitter{
		AccessToken:       "very bad",
		AccessTokenSecret: "bad ",
		ConsumerKey:       "bad",
		ConsumerSecret:    "bad",
		reqSigner: &inet.OAuthSigner{
			AccessToken: "bad",
		},
	}
	apps.TwoFACodeGenerator.SecretFile.FilePath = "does not exist"
	apps.WolframAlpha.AppID = "very bad"

	// Very few apps discover configuration error during initialisation
	initErr := apps.Initialise()
	t.Logf("Initialisation discoveries: %+v", initErr)
	findAllInitErrs := StringContainsAllOf(initErr.Error(), []Trigger{
		"AESEncryptedFile",
		"TextSearch",
		"TwoFA",
	})
	if findAllInitErrs != nil {
		t.Fatal(findAllInitErrs)
	}
	// Apps that fail initialisation remain disabled

	// Majority of the apps discover configuration error during self test
	selfTestErr := apps.SelfTest()
	t.Logf("Self test discoveries: %+v", selfTestErr)
	findAllSelfTestErrs := StringContainsAllOf(selfTestErr.Error(), []Trigger{
		"IMAPAccounts",
		"RSS",
		"SendMail",
		"Shell",
		"Twilio",
		"Twitter",
		"WolframAlpha",
	})
	if findAllSelfTestErrs != nil {
		t.Fatal(findAllSelfTestErrs)
	}
}
