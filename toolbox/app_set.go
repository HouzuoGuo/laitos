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
	LookupByTrigger map[Trigger]Feature `json:"-"`

	AESDecrypt         AESDecrypt         `json:"AESDecrypt"`
	BrowserPhantomJS   BrowserPhantomJS   `json:"BrowserPhantomJS"`
	BrowserSlimerJS    BrowserSlimerJS    `json:"BrowserSlimerJS"`
	PublicContact      PublicContact      `json:"PublicContact"`
	EnvControl         EnvControl         `json:"EnvControl"`
	IMAPAccounts       IMAPAccounts       `json:"IMAPAccounts"`
	Joke               Joke               `json:"Joke"`
	RSS                RSS                `json:"RSS"`
	SendMail           SendMail           `json:"SendMail"`
	Shell              Shell              `json:"Shell"`
	TextSearch         TextSearch         `json:"TextSearch"`
	Twilio             Twilio             `json:"Twilio"`
	Twitter            Twitter            `json:"Twitter"`
	TwoFACodeGenerator TwoFACodeGenerator `json:"TwoFACodeGenerator"`
	WolframAlpha       WolframAlpha       `json:"WolframAlpha"`

	MessageProcessor MessageProcessor `json:"MessageProcessor"`
}

//var TestFeatureSet = FeatureSet{} // Features are assigned by init_test.go

// Run initialisation routine on all features, and then populate lookup table for all configured features.
func (fs *FeatureSet) Initialise() error {
	fs.LookupByTrigger = map[Trigger]Feature{}
	triggers := map[Trigger]Feature{
		fs.AESDecrypt.Trigger():         &fs.AESDecrypt,         // a
		fs.BrowserPhantomJS.Trigger():   &fs.BrowserPhantomJS,   // bp
		fs.BrowserSlimerJS.Trigger():    &fs.BrowserSlimerJS,    // bs
		fs.PublicContact.Trigger():      &fs.PublicContact,      // c
		fs.EnvControl.Trigger():         &fs.EnvControl,         // e
		fs.TextSearch.Trigger():         &fs.TextSearch,         // g
		fs.IMAPAccounts.Trigger():       &fs.IMAPAccounts,       // i
		fs.Joke.Trigger():               &fs.Joke,               // j
		fs.RSS.Trigger():                &fs.RSS,                // r
		fs.SendMail.Trigger():           &fs.SendMail,           // m
		fs.Shell.Trigger():              &fs.Shell,              // s
		fs.Twilio.Trigger():             &fs.Twilio,             // p
		fs.Twitter.Trigger():            &fs.Twitter,            // t
		fs.TwoFACodeGenerator.Trigger(): &fs.TwoFACodeGenerator, // 2
		fs.WolframAlpha.Trigger():       &fs.WolframAlpha,       // w

		fs.MessageProcessor.Trigger(): &fs.MessageProcessor, // 0m
	}
	errs := make([]string, 0)
	for trigger, featureRef := range triggers {
		/*
			Collect initialisation errors (if any) from all failed features so that all mistakes can be presented to
			caller at once.
		*/
		if featureRef.IsConfigured() {
			if err := featureRef.Initialise(); err != nil {
				errs = append(errs, err.Error())
			}
			fs.LookupByTrigger[trigger] = featureRef
		}
	}
	if len(errs) != 0 {
		return errors.New(strings.Join(errs, " | "))
	}
	return nil
}

// Run self test of all configured features in parallel. Return test errors if any.
func (fs *FeatureSet) SelfTest() error {
	ret := make([]string, 0)
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
		"BrowserPhantomJS":   &fs.BrowserPhantomJS,
		"BrowserSlimerJS":    &fs.BrowserSlimerJS,
		"EnvControl":         &fs.EnvControl,
		"IMAPAccounts":       &fs.IMAPAccounts,
		"Joke":               &fs.Joke,
		"RSS":                &fs.RSS,
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
