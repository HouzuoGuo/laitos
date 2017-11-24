package toolbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

/*
FeatureSet contains an instance of every available toolbox feature. Together they may initialise, run self tests,
and receive configuration from JSON.
*/
type FeatureSet struct {
	AESDecrypt         AESDecrypt          `json:"AESDecrypt"`
	Browser            Browser             `json:"Browser"`
	PublicContact      PublicContact       `json:"PublicContact"`
	EnvControl         EnvControl          `json:"EnvControl"`
	Facebook           Facebook            `json:"Facebook"`
	IMAPAccounts       IMAPAccounts        `json:"IMAPAccounts"`
	SendMail           SendMail            `json:"SendMail"`
	Shell              Shell               `json:"Shell"`
	Twilio             Twilio              `json:"Twilio"`
	Twitter            Twitter             `json:"Twitter"`
	TwoFACodeGenerator TwoFACodeGenerator  `json:"TwoFACodeGenerator"`
	WolframAlpha       WolframAlpha        `json:"WolframAlpha"`
	LookupByTrigger    map[Trigger]Feature `json:"-"`
}

var TestFeatureSet = FeatureSet{} // Features are assigned by init_test.go

// Run initialisation routine on all features, and then populate lookup table for all configured features.
func (fs *FeatureSet) Initialise() error {
	fs.LookupByTrigger = map[Trigger]Feature{}
	triggers := map[Trigger]Feature{
		fs.AESDecrypt.Trigger():         &fs.AESDecrypt,         // a
		fs.Browser.Trigger():            &fs.Browser,            // b
		fs.PublicContact.Trigger():      &fs.PublicContact,      // c
		fs.EnvControl.Trigger():         &fs.EnvControl,         // e
		fs.Facebook.Trigger():           &fs.Facebook,           // f
		fs.IMAPAccounts.Trigger():       &fs.IMAPAccounts,       // i
		fs.SendMail.Trigger():           &fs.SendMail,           // m
		fs.Shell.Trigger():              &fs.Shell,              // s
		fs.Twilio.Trigger():             &fs.Twilio,             // p
		fs.Twitter.Trigger():            &fs.Twitter,            // t
		fs.TwoFACodeGenerator.Trigger(): &fs.TwoFACodeGenerator, // 2
		fs.WolframAlpha.Trigger():       &fs.WolframAlpha,       // w
	}
	for trigger, featureRef := range triggers {
		if featureRef.IsConfigured() {
			if err := featureRef.Initialise(); err != nil {
				return err
			}
			fs.LookupByTrigger[trigger] = featureRef
		}
	}
	return nil
}

// Run self test of all configured features in parallel. Return test errors if any.
func (fs *FeatureSet) SelfTest() error {
	ret := make([]string, 0, 0)
	retMutex := &sync.Mutex{}
	wait := &sync.WaitGroup{}
	wait.Add(len(fs.LookupByTrigger))
	for _, featureRef := range fs.LookupByTrigger {
		go func(ref Feature) {
			err := ref.SelfTest()
			if err != nil {
				retMutex.Lock()
				ret = append(ret, fmt.Sprintf("%s: %s", ref.Trigger(), err.Error()))
				retMutex.Unlock()
			}
			wait.Done()
		}(featureRef)
	}
	wait.Wait()
	if len(ret) == 0 {
		return nil
	}
	return errors.New(strings.Join(ret, " | "))
}

// Deserialise feature configuration from JSON configuration. The function does not initialise features automatically.
func (fs *FeatureSet) DeserialiseFromJSON(configJSON json.RawMessage) error {
	// Turn input JSON into map[string]json.RawMessage, map key is the feature key in JSON.
	var configMap map[string]json.RawMessage
	if err := json.Unmarshal(configJSON, &configMap); err != nil {
		return fmt.Errorf("FeatureSet.DeserialiseFromJSON: failed to retrieve config map - %v", err)
	}
	// Here are the feature keys
	features := map[string]Feature{
		"AESDecrypt":         &fs.AESDecrypt,
		"Browser":            &fs.Browser,
		"EnvControl":         &fs.EnvControl,
		"Facebook":           &fs.Facebook,
		"IMAPAccounts":       &fs.IMAPAccounts,
		"SendMail":           &fs.SendMail,
		"Shell":              &fs.Shell,
		"Twilio":             &fs.Twilio,
		"Twitter":            &fs.Twitter,
		"TwoFACodeGenerator": &fs.TwoFACodeGenerator,
		"WolframAlpha":       &fs.WolframAlpha,
	}
	for featureKey, featureRef := range features {
		if featureJSON, exists := configMap[featureKey]; exists {
			if err := json.Unmarshal(featureJSON, &featureRef); err != nil {
				return fmt.Errorf("FeatureSet.DeserialiseFromJSON: failed to deserialise JSON key %s - %v", featureKey, err)
			}
		}
	}
	return nil
}

// Return all configured & initialised triggers, sorted in alphabetical order.
func (fs *FeatureSet) GetTriggers() []string {
	ret := make([]string, 0, 8)
	if fs.LookupByTrigger == nil {
		return ret
	}
	for trigger := range fs.LookupByTrigger {
		ret = append(ret, string(trigger))
	}
	sort.Strings(ret)
	return ret
}
