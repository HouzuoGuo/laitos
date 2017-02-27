package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/HouzuoGuo/websh/bridge"
	"github.com/HouzuoGuo/websh/email"
	"github.com/HouzuoGuo/websh/feature"
	"github.com/HouzuoGuo/websh/frontend/common"
	"github.com/HouzuoGuo/websh/frontend/httpd"
	"github.com/HouzuoGuo/websh/frontend/httpd/api"
	"github.com/HouzuoGuo/websh/frontend/mailp"
	"log"
)

// Configuration of a standard set of bridges that are useful to both HTTP daemon and mail processor.
type StandardBridges struct {
	// Before command...
	TranslateSequences bridge.TranslateSequences `json:"TranslateSequences"`
	PINAndShortcuts    bridge.PINAndShortcuts    `json:"PINAndShortcuts"`

	// After result...
	NotifyViaEmail bridge.NotifyViaEmail `json:"NotifyViaEmail"`
	LintText       bridge.LintText       `json:"LintText"`
}

// Configure path to HTTP handlers and handler themselves.
type HTTPHandlers struct {
	SelfTestEndpoint string `json:"SelfTestEndpoint"`

	TwilioSMSEndpoint        string                   `json:"TwilioSMSEndpoint"`
	TwilioCallEndpoint       string                   `json:"TwilioCallEndpoint"`
	TwilioCallEndpointConfig api.HandleTwilioCallHook `json:"TwilioCallEndpointConfig"`

	MailMeEndpoint       string           `json:"MailMeEndpoint"`
	MailMeEndpointConfig api.HandleMailMe `json:"MailMeEndpointConfig"`
}

// The structure is JSON-compatible and capable of setting up all features and front-end services.
type Config struct {
	Features             feature.FeatureSet  `json:"Features"`             // Feature configuration is shared by all services
	HTTPDaemon           httpd.HTTPD         `json:"HTTPDaemon"`           // HTTP daemon configuration
	HTTPBridges          StandardBridges     `json:"HTTPBridges"`          // HTTP daemon bridge configuration
	HTTPHandlers         HTTPHandlers        `json:"HTTPHandlers"`         // HTTP daemon handler configuration
	MailProcessor        mailp.MailProcessor `json:"MailProcessor"`        // Incoming mail processor configuration
	MailProcessorBridges StandardBridges     `json:"MailProcessorBridges"` // Incoming mail processor bridge configuration
	Mailer               email.Mailer        `json:"Mailer"`               // Outgoing mail configuration for notifications and mail replies
}

// Deserialise JSON data into config structures.
func (config *Config) DeserialiseFromJSON(in []byte) error {
	return json.Unmarshal(in, config)
}

// Construct an HTTP daemon from configuration and return.
func (config *Config) GetHTTPD() *httpd.HTTPD {
	ret := config.HTTPDaemon

	mailNotification := config.HTTPBridges.NotifyViaEmail
	mailNotification.Mailer = &config.Mailer
	features := config.Features
	if err := features.Initialise(); err != nil {
		log.Panicf("GetHTTPD: failed to initialise features - %v", err)
	}
	// Assemble command processor from features and bridges
	ret.Processor = &common.CommandProcessor{
		Features: &features,
		CommandBridges: []bridge.CommandBridge{
			&config.HTTPBridges.PINAndShortcuts,
			&config.HTTPBridges.TranslateSequences,
		},
		ResultBridges: []bridge.ResultBridge{
			&bridge.ResetCombinedText{}, // this is mandatory but not configured by user's config file
			&bridge.LintText{TrimSpaces: true, MaxLength: 35},
			&bridge.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
			&mailNotification,
		},
	}
	// Make handler factories
	handlers := map[string]api.HandlerFactory{}
	if config.HTTPHandlers.SelfTestEndpoint != "" {
		handlers[config.HTTPHandlers.SelfTestEndpoint] = &api.HandleFeatureSelfTest{}
		log.Print("GetHTTPD: feature self-test endpoint is enabled")
	}
	if config.HTTPHandlers.TwilioSMSEndpoint != "" {
		handlers[config.HTTPHandlers.TwilioSMSEndpoint] = &api.HandleTwilioSMSHook{}
		log.Print("GetHTTPD: Twilio SMS hook endpoint is enabled")
	}
	if config.HTTPHandlers.TwilioCallEndpoint != "" {
		/*
		 Configure a callback endpoint for Twilio call's callback.
		 The endpoint name is automatically generated from random bytes.
		*/
		randBytes := make([]byte, 16)
		_, err := rand.Read(randBytes)
		if err != nil {
			log.Panicf("GetHTTPD: failed to read random number - %v", err)
		}
		callbackEndpoint := "/" + hex.EncodeToString(randBytes)
		// The greeting handler will use the callback endpoint to handle command
		config.HTTPHandlers.TwilioCallEndpointConfig.CallbackEndpoint = callbackEndpoint
		handlers[config.HTTPHandlers.TwilioCallEndpoint] = &config.HTTPHandlers.TwilioCallEndpointConfig
		// The callback handler will use the callback point that points to itself to carry on with phone conversation
		handlers[callbackEndpoint] = &api.HandleTwilioCallCallback{MyEndpoint: callbackEndpoint}
		log.Print("GetHTTPD: Twilio call hook endpoint is enabled")
	}
	if config.HTTPHandlers.MailMeEndpoint != "" {
		handler := config.HTTPHandlers.MailMeEndpointConfig
		handler.Mailer = &config.Mailer
		handlers[config.HTTPHandlers.MailMeEndpoint] = &handler
		log.Print("GetHTTPD: MailMe endpoint is enabled")
	}
	ret.Handlers = handlers
	return &ret
}

// Construct a mail processor from configuration and return.
func (config *Config) GetMailProcessor() *mailp.MailProcessor {
	ret := config.MailProcessor

	mailNotification := config.MailProcessorBridges.NotifyViaEmail
	mailNotification.Mailer = &config.Mailer
	features := config.Features
	if err := features.Initialise(); err != nil {
		log.Panicf("GetMailProcessor: failed to initialise features - %v", err)
	}
	// Assemble command processor from features and bridges
	ret.Processor = &common.CommandProcessor{
		Features: &features,
		CommandBridges: []bridge.CommandBridge{
			&config.MailProcessorBridges.PINAndShortcuts,
			&config.MailProcessorBridges.TranslateSequences,
		},
		ResultBridges: []bridge.ResultBridge{
			&bridge.ResetCombinedText{}, // this is mandatory but not configured by user's config file
			&bridge.LintText{TrimSpaces: true, MaxLength: 35},
			&bridge.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
			&mailNotification,
		},
	}
	ret.ReplyMailer = &config.Mailer
	return &ret
}
