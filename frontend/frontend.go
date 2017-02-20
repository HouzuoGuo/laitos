// Daemon services that act as IO front-end to features, while using bridges to transform IO content.
package frontend

import (
	"encoding/json"
	"fmt"
	"github.com/HouzuoGuo/websh/feature"
	"log"
)

// Aggregate all available features together.
type FeatureSet struct {
	Facebook     feature.Facebook
	Shell        feature.Shell
	Twilio       feature.Twilio
	Twitter      feature.Twitter
	WolframAlpha feature.WolframAlpha
	Undocumented1 feature.Undocumented1
}

// Initialise all features from JSON configuration. If a feature's JSON key is missing, the feature won't be initialised.
func (fs *FeatureSet) Initialise(config map[string]json.RawMessage) error {
	features := map[string]feature.Feature{
		"Facebook":     &fs.Facebook,
		"Shell":        &fs.Shell,
		"Twilio":       &fs.Twilio,
		"Twitter":      &fs.Twitter,
		"WolframAlpha": &fs.WolframAlpha,
		"Undocumented1": &fs.Undocumented1,
	}
	// Deserialise configuration of individual feature
	for jsonKey, featureRef := range features {
		if conf, found := config[jsonKey]; found {
			if err := json.Unmarshal(conf, featureRef); err != nil {
				return fmt.Errorf("FeatureSet.Initialise: JSON error in key \"%s\" - %v", jsonKey, err)
			}
		}
	}
	// Initialise all features
	for jsonKey, featureRef := range features {
		if featureRef.IsConfigured() {
			if err := featureRef.Initialise(); err != nil {
				return err
			}
		} else {
			log.Printf("FeatureSet.Initialise: skip initialisation of feature \"%s\" due to missing configuration", jsonKey)
		}
	}
	return nil
}

// A front-end daemon is capable of starting, stopping, self-testing.
type FrontendDaemon interface {
	InitialiseByJSON(json.RawMessage) error
	StartAndBlock() error
	SelfTest() error
	Stop() error
}
