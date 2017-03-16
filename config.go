package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/HouzuoGuo/laitos/bridge"
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/frontend/dnsd"
	"github.com/HouzuoGuo/laitos/frontend/httpd"
	"github.com/HouzuoGuo/laitos/frontend/httpd/api"
	"github.com/HouzuoGuo/laitos/frontend/mailp"
	"github.com/HouzuoGuo/laitos/frontend/smtpd"
	"github.com/HouzuoGuo/laitos/frontend/telegram_bot"
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
	SelfTestEndpoint    string `json:"SelfTestEndpoint"`
	InformationEndpoint string `json:"InformationEndpoint"`

	CommandFormEndpoint string `json:"CommandFormEndpoint"`

	IndexEndpoints      []string               `json:"IndexEndpoints"`
	IndexEndpointConfig api.HandleHTMLDocument `json:"IndexEndpointConfig"`

	MailMeEndpoint       string           `json:"MailMeEndpoint"`
	MailMeEndpointConfig api.HandleMailMe `json:"MailMeEndpointConfig"`

	WebProxyEndpoint string `json:"WebProxyEndpoint"`

	TwilioSMSEndpoint        string                   `json:"TwilioSMSEndpoint"`
	TwilioCallEndpoint       string                   `json:"TwilioCallEndpoint"`
	TwilioCallEndpointConfig api.HandleTwilioCallHook `json:"TwilioCallEndpointConfig"`
}

// The structure is JSON-compatible and capable of setting up all features and front-end services.
type Config struct {
	Features feature.FeatureSet `json:"Features"` // Feature configuration is shared by all services
	Mailer   email.Mailer       `json:"Mailer"`   // Mail configuration for notifications and mail processor results

	DNSDaemon dnsd.DNSD `json:"DNSDaemon"` // DNS daemon configuration

	HTTPDaemon   httpd.HTTPD     `json:"HTTPDaemon"`   // HTTP daemon configuration
	HTTPBridges  StandardBridges `json:"HTTPBridges"`  // HTTP daemon bridge configuration
	HTTPHandlers HTTPHandlers    `json:"HTTPHandlers"` // HTTP daemon handler configuration

	MailDaemon           smtpd.SMTPD         `json:"MailDaemon"`           // SMTP daemon configuration
	MailProcessor        mailp.MailProcessor `json:"MailProcessor"`        // Incoming mail processor configuration
	MailProcessorBridges StandardBridges     `json:"MailProcessorBridges"` // Incoming mail processor bridge configuration

	TelegramBot        telegram.TelegramBot `json:"TelegramBot"`        // Telegram bot configuration
	TelegramBotBridges StandardBridges      `json:"TelegramBotBridges"` // Telegram bot bridge configuration
}

// Deserialise JSON data into config structures.
func (config *Config) DeserialiseFromJSON(in []byte) error {
	if err := json.Unmarshal(in, config); err != nil {
		return err
	}
	return nil
}

// Construct a DNS daemon from configuration and return.
func (config *Config) GetDNSD() *dnsd.DNSD {
	ret := config.DNSDaemon
	if err := ret.Initialise(); err != nil {
		log.Fatalf("Config.GetDNSD: failed to initialise - %v", err)
		return nil
	}
	return &ret
}

// Construct an HTTP daemon from configuration and return.
func (config *Config) GetHTTPD() *httpd.HTTPD {
	ret := config.HTTPDaemon

	mailNotification := config.HTTPBridges.NotifyViaEmail
	mailNotification.Mailer = config.Mailer
	features := config.Features
	if err := features.Initialise(); err != nil {
		log.Fatalf("Config.GetHTTPD: failed to initialise features - %v", err)
		return nil
	}
	log.Printf("Config.GetHTTPD: enabled features are - %v", features.GetTriggers())
	// Assemble command processor from features and bridges
	ret.Processor = &common.CommandProcessor{
		Features: &features,
		CommandBridges: []bridge.CommandBridge{
			&config.HTTPBridges.PINAndShortcuts,
			&config.HTTPBridges.TranslateSequences,
		},
		ResultBridges: []bridge.ResultBridge{
			&bridge.ResetCombinedText{}, // this is mandatory but not configured by user's config file
			&config.HTTPBridges.LintText,
			&bridge.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
			&mailNotification,
		},
	}
	// Make handler factories
	handlers := map[string]api.HandlerFactory{}
	if config.HTTPHandlers.SelfTestEndpoint != "" {
		handlers[config.HTTPHandlers.SelfTestEndpoint] = &api.HandleFeatureSelfTest{}
	}
	if config.HTTPHandlers.InformationEndpoint != "" {
		handlers[config.HTTPHandlers.InformationEndpoint] = &api.HandleSystemInfo{}
	}
	if config.HTTPHandlers.CommandFormEndpoint != "" {
		handlers[config.HTTPHandlers.CommandFormEndpoint] = &api.HandleCommandForm{}
	}
	if config.HTTPHandlers.IndexEndpoints != nil {
		for _, location := range config.HTTPHandlers.IndexEndpoints {
			handlers[location] = &config.HTTPHandlers.IndexEndpointConfig
		}
	}
	if config.HTTPHandlers.MailMeEndpoint != "" {
		handler := config.HTTPHandlers.MailMeEndpointConfig
		handler.Mailer = config.Mailer
		handlers[config.HTTPHandlers.MailMeEndpoint] = &handler
	}
	if proxyEndpoint := config.HTTPHandlers.WebProxyEndpoint; proxyEndpoint != "" {
		handlers[proxyEndpoint] = &api.HandleWebProxy{MyEndpoint: proxyEndpoint}
	}
	if config.HTTPHandlers.TwilioSMSEndpoint != "" {
		handlers[config.HTTPHandlers.TwilioSMSEndpoint] = &api.HandleTwilioSMSHook{}
	}
	if config.HTTPHandlers.TwilioCallEndpoint != "" {
		/*
		 Configure a callback endpoint for Twilio call's callback.
		 The endpoint name is automatically generated from random bytes.
		*/
		randBytes := make([]byte, 32)
		_, err := rand.Read(randBytes)
		if err != nil {
			log.Panicf("Config.GetHTTPD: failed to read random number - %v", err)
		}
		callbackEndpoint := "/" + hex.EncodeToString(randBytes)
		// The greeting handler will use the callback endpoint to handle command
		config.HTTPHandlers.TwilioCallEndpointConfig.CallbackEndpoint = callbackEndpoint
		handlers[config.HTTPHandlers.TwilioCallEndpoint] = &config.HTTPHandlers.TwilioCallEndpointConfig
		// The callback handler will use the callback point that points to itself to carry on with phone conversation
		handlers[callbackEndpoint] = &api.HandleTwilioCallCallback{MyEndpoint: callbackEndpoint}
	}
	ret.SpecialHandlers = handlers
	// Call initialise and print out prefixes of installed routes
	if err := ret.Initialise(); err != nil {
		log.Fatalf("Config.GetHTTPD: failed to initialise - %v", err)
		return nil
	}
	for route := range ret.AllRoutes {
		shortRoute := route
		if len(route) > 12 {
			shortRoute = route[0:12] + "..."
		}
		log.Printf("Config.GetHTTPD: installed route %s", shortRoute)
	}
	return &ret
}

/*
Construct a mail processor from configuration and return. The mail processor will use the common mailer to send replies.
Mail processor is usually built into laitos' own SMTP daemon to process feature commands from incoming mails, but an
independent mail process is useful in certain scenarios, such as integrating with postfix's "forward-mail-to-program"
mechanism.
*/
func (config *Config) GetMailProcessor() *mailp.MailProcessor {
	ret := config.MailProcessor

	mailNotification := config.MailProcessorBridges.NotifyViaEmail
	mailNotification.Mailer = config.Mailer
	features := config.Features
	if err := features.Initialise(); err != nil {
		log.Fatalf("Config.GetMailProcessor: failed to initialise features - %v", err)
		return nil
	}
	log.Printf("Config.GetMailProcessor: enabled features are - %v", features.GetTriggers())
	// Assemble command processor from features and bridges
	ret.Processor = &common.CommandProcessor{
		Features: &features,
		CommandBridges: []bridge.CommandBridge{
			&config.MailProcessorBridges.PINAndShortcuts,
			&config.MailProcessorBridges.TranslateSequences,
		},
		ResultBridges: []bridge.ResultBridge{
			&bridge.ResetCombinedText{}, // this is mandatory but not configured by user's config file
			&config.MailProcessorBridges.LintText,
			&bridge.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
			&mailNotification,
		},
	}
	ret.ReplyMailer = config.Mailer
	return &ret
}

/*
Construct an SMTP daemon together with its mail command processor.
Both SMTP daemon and mail command processor will use the common mailer to forward mails and send replies.
*/
func (config *Config) GetMailDaemon() *smtpd.SMTPD {
	ret := config.MailDaemon
	ret.MailProcessor = config.GetMailProcessor()
	ret.ForwardMailer = config.Mailer
	if err := ret.Initialise(); err != nil {
		log.Fatalf("Config.GetMailDeamon: failed to initialise - %v", err)
		return nil
	}
	return &ret
}

// Construct a telegram bot from configuration and return.
func (config *Config) GetTelegramBot() *telegram.TelegramBot {
	ret := config.TelegramBot

	mailNotification := config.TelegramBotBridges.NotifyViaEmail
	mailNotification.Mailer = config.Mailer
	features := config.Features
	if err := features.Initialise(); err != nil {
		log.Fatalf("Config.GetTelegramBot: failed to initialise features - %v", err)
		return nil
	}
	log.Printf("Config.GetTelegramBot: enabled features are - %v", features.GetTriggers())
	// Assemble telegram bot from features and bridges
	ret.Processor = &common.CommandProcessor{
		Features: &features,
		CommandBridges: []bridge.CommandBridge{
			&config.TelegramBotBridges.PINAndShortcuts,
			&config.TelegramBotBridges.TranslateSequences,
		},
		ResultBridges: []bridge.ResultBridge{
			&bridge.ResetCombinedText{}, // this is mandatory but not configured by user's config file
			&config.TelegramBotBridges.LintText,
			&bridge.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
			&mailNotification,
		},
	}
	return &ret
}
