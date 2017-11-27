package launcher

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/daemon/httpd"
	"github.com/HouzuoGuo/laitos/daemon/httpd/handler"
	"github.com/HouzuoGuo/laitos/daemon/maintenance"
	"github.com/HouzuoGuo/laitos/daemon/plainsocket"
	"github.com/HouzuoGuo/laitos/daemon/smtpd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/daemon/sockd"
	"github.com/HouzuoGuo/laitos/daemon/telegrambot"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/HouzuoGuo/laitos/toolbox/filter"
	"sync"
)

/*
StandardFilters contains a standard set of filters (PIN match, notification, lint, etc) that are useful to nearly all
laitos daemons that carry a command processor. The filters' configuration is fed by Config struct, which is itself fed
by deserialised JSON text.
*/
type StandardFilters struct {
	// For input command content
	TranslateSequences filter.TranslateSequences `json:"TranslateSequences"`
	PINAndShortcuts    filter.PINAndShortcuts    `json:"PINAndShortcuts"`

	// For command execution result
	NotifyViaEmail filter.NotifyViaEmail `json:"NotifyViaEmail"`
	LintText       filter.LintText       `json:"LintText"`
}

// Configure path to HTTP handlers and handler themselves.
type HTTPHandlers struct {
	InformationEndpoint string `json:"InformationEndpoint"`

	BrowserEndpoint       string                `json:"BrowserEndpoint"`
	BrowserEndpointConfig handler.HandleBrowser `json:"BrowserEndpointConfig"`

	CommandFormEndpoint string `json:"CommandFormEndpoint"`

	GitlabBrowserEndpoint       string                      `json:"GitlabBrowserEndpoint"`
	GitlabBrowserEndpointConfig handler.HandleGitlabBrowser `json:"GitlabBrowserEndpointConfig"`

	IndexEndpoints      []string                   `json:"IndexEndpoints"`
	IndexEndpointConfig handler.HandleHTMLDocument `json:"IndexEndpointConfig"`

	MailMeEndpoint       string               `json:"MailMeEndpoint"`
	MailMeEndpointConfig handler.HandleMailMe `json:"MailMeEndpointConfig"`

	MicrosoftBotEndpoint1       string                     `json:"MicrosoftBotEndpoint1"`
	MicrosoftBotEndpointConfig1 handler.HandleMicrosoftBot `json:"MicrosoftBotEndpointConfig1"`
	MicrosoftBotEndpoint2       string                     `json:"MicrosoftBotEndpoint2"`
	MicrosoftBotEndpointConfig2 handler.HandleMicrosoftBot `json:"MicrosoftBotEndpointConfig2"`
	MicrosoftBotEndpoint3       string                     `json:"MicrosoftBotEndpoint3"`
	MicrosoftBotEndpointConfig3 handler.HandleMicrosoftBot `json:"MicrosoftBotEndpointConfig3"`

	WebProxyEndpoint string `json:"WebProxyEndpoint"`

	TwilioSMSEndpoint        string                       `json:"TwilioSMSEndpoint"`
	TwilioCallEndpoint       string                       `json:"TwilioCallEndpoint"`
	TwilioCallEndpointConfig handler.HandleTwilioCallHook `json:"TwilioCallEndpointConfig"`
}

// The structure is JSON-compatible and capable of setting up all features and front-end services.
type Config struct {
	/*
		Features are toolbox feature instances shared by all daemons and command runners. Avoid duplicating this
		structure because certain toolbox features (such as AES file decryption) may hold large amount of data in
		memory. Therefore, all daemon preparation and initialisation routines operate on reference to this FeatureSet.
	*/
	Features   *toolbox.FeatureSet `json:"Features"`
	MailClient inet.MailClient     `json:"MailClient"` // MailClient is the common client configuration for sending notification emails and mail command runner results.

	Maintenance *maintenance.Daemon `json:"Maintenance"` // Daemon configures behaviour of periodic health-check/system maintenance

	DNSDaemon *dnsd.Daemon `json:"DNSDaemon"` // DNS daemon configuration

	HTTPDaemon   *httpd.Daemon   `json:"HTTPDaemon"`   // HTTP daemon configuration
	HTTPFilters  StandardFilters `json:"HTTPFilters"`  // HTTP daemon filter configuration
	HTTPHandlers HTTPHandlers    `json:"HTTPHandlers"` // HTTP daemon handler configuration

	MailDaemon        *smtpd.Daemon          `json:"MailDaemon"`        // SMTP daemon configuration
	MailCommandRunner *mailcmd.CommandRunner `json:"MailCommandRunner"` // MailCommandRunner processes toolbox commands from incoming mail body.

	MailFilters StandardFilters `json:"MailFilters"` // MailFilters configure command processor for mail command runner

	PlainSocketDaemon  *plainsocket.Daemon `json:"PlainSocketDaemon"`  // Plain text protocol TCP and UDP daemon configuration
	PlainSocketFilters StandardFilters     `json:"PlainSocketFilters"` // Plain text daemon filter configuration

	SockDaemon *sockd.Daemon `json:"SockDaemon"` // Intentionally undocumented

	TelegramBot     *telegrambot.Daemon `json:"TelegramBot"`     // Telegram bot configuration
	TelegramFilters StandardFilters     `json:"TelegramFilters"` // Telegram bot filter configuration

	SupervisorNotificationRecipients []string `json:"SupervisorNotificationRecipients"` // Email addresses of supervisor notification recipients

	logger                misc.Logger // logger handles log output from configuration serialisation and initialisation routines.
	maintenanceInit       sync.Once
	dnsDaemonInit         sync.Once
	httpDaemonInit        sync.Once
	mailCommandRunnerInit sync.Once
	mailDaemonInit        sync.Once
	plainSocketDaemonInit sync.Once
	sockDaemonInit        sync.Once
	telegramBotInit       sync.Once
}

// Initialise decorates feature configuration and bridges in preparation for daemon operations.
func (config *Config) Initialise() error {
	/*
		Fill in some blanks so that Get**** functions will be able to call Initialise() function, which in turn
		returns an error with meaningful message telling user that daemon is lacking configuration and will not start.
		If the nil daemons are not empty, user will only see a panic caused by nil, which is not very helpful.

		For the config.Features case, an empty FeatureSet can still offer several useful features such as program
		environment control and public institution contacts.
	*/
	if config.Features == nil {
		config.Features = &toolbox.FeatureSet{}
	}
	if config.DNSDaemon == nil {
		config.DNSDaemon = &dnsd.Daemon{}
	}
	if config.HTTPDaemon == nil {
		config.HTTPDaemon = &httpd.Daemon{}
	}
	if config.MailCommandRunner == nil {
		config.MailCommandRunner = &mailcmd.CommandRunner{}
	}
	if config.MailDaemon == nil {
		config.MailDaemon = &smtpd.Daemon{}
	}
	if config.PlainSocketDaemon == nil {
		config.PlainSocketDaemon = &plainsocket.Daemon{}
	}
	if config.SockDaemon == nil {
		config.SockDaemon = &sockd.Daemon{}
	}
	if config.TelegramBot == nil {
		config.TelegramBot = &telegrambot.Daemon{}
	}
	// All notification filters share the common mail client
	config.HTTPFilters.NotifyViaEmail.MailClient = config.MailClient
	config.MailFilters.NotifyViaEmail.MailClient = config.MailClient
	config.PlainSocketFilters.NotifyViaEmail.MailClient = config.MailClient
	config.TelegramFilters.NotifyViaEmail.MailClient = config.MailClient
	// SendMail feature also shares the common mail client
	config.Features.SendMail.MailClient = config.MailClient
	if err := config.Features.Initialise(); err != nil {
		return err
	}
	config.logger.Printf("Initialise", "", nil, "enabled features are - %v", config.Features.GetTriggers())
	return nil
}

/*
DeserialiseFromJSON deserialised configuration of all daemons and toolbox features from JSON input, and then prepares
itself for daemon operations.
*/
func (config *Config) DeserialiseFromJSON(in []byte) error {
	config.logger = misc.Logger{ComponentName: "Config"}
	if err := json.Unmarshal(in, config); err != nil {
		return err
	}
	if err := config.Initialise(); err != nil {
		return err
	}
	return nil
}

// Construct a DNS daemon from configuration and return.
func (config *Config) GetDNSD() *dnsd.Daemon {
	config.dnsDaemonInit.Do(func() {
		if err := config.DNSDaemon.Initialise(); err != nil {
			config.logger.Fatalf("GetDNSD", "", err, "failed to initialise")
			return
		}
	})
	return config.DNSDaemon
}

// GetMaintenance constructs a system maintenance / health check daemon from configuration and return.
func (config *Config) GetMaintenance() *maintenance.Daemon {
	config.maintenanceInit.Do(func() {
		config.Maintenance.FeaturesToTest = config.Features
		config.Maintenance.MailClient = config.MailClient
		config.Maintenance.MailCmdRunnerToTest = config.GetMailCommandRunner()
		config.Maintenance.HTTPHandlersToCheck = config.GetHTTPD().HandlerCollection
		if err := config.Maintenance.Initialise(); err != nil {
			config.logger.Fatalf("GetMaintenance", "", err, "failed to initialise")
			return
		}
	})
	return config.Maintenance
}

// Construct an HTTP daemon from configuration and return.
func (config *Config) GetHTTPD() *httpd.Daemon {
	config.httpDaemonInit.Do(func() {
		// Assemble command processor from features and filters
		config.HTTPDaemon.Processor = &common.CommandProcessor{
			Features: config.Features,
			CommandFilters: []filter.CommandFilter{
				&config.HTTPFilters.PINAndShortcuts,
				&config.HTTPFilters.TranslateSequences,
			},
			ResultFilters: []filter.ResultFilter{
				&filter.ResetCombinedText{}, // this is mandatory but not configured by user's config file
				&config.HTTPFilters.LintText,
				&filter.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
				&config.HTTPFilters.NotifyViaEmail,
			},
		}
		// Make handler factories
		handlers := httpd.HandlerCollection{}
		if config.HTTPHandlers.InformationEndpoint != "" {
			handlers[config.HTTPHandlers.InformationEndpoint] = &handler.HandleSystemInfo{
				FeaturesToCheck: config.Features,
				// Caller is not going to manipulate with acquired mail processor, so my instance is going to be identical to caller's.
				CheckMailCmdRunner: config.GetMailCommandRunner(),
			}
		}
		if config.HTTPHandlers.BrowserEndpoint != "" {
			/*
			 Configure a browser image endpoint for browser page.
			 The endpoint name is automatically generated from random bytes.
			*/
			randBytes := make([]byte, 32)
			_, err := rand.Read(randBytes)
			if err != nil {
				config.logger.Fatalf("GetHTTPD", "", err, "failed to read random number")
				return
			}
			// Image handler needs to operate on browser handler's browser instances
			browserImageHandler := &handler.HandleBrowserImage{}
			browserHandler := config.HTTPHandlers.BrowserEndpointConfig
			imageEndpoint := "/" + hex.EncodeToString(randBytes)
			handlers[imageEndpoint] = browserImageHandler
			// Browser handler needs to use image handler's path
			browserHandler.ImageEndpoint = imageEndpoint
			browserImageHandler.Browsers = &browserHandler.Browsers
			handlers[config.HTTPHandlers.BrowserEndpoint] = &browserHandler
		}
		if config.HTTPHandlers.CommandFormEndpoint != "" {
			handlers[config.HTTPHandlers.CommandFormEndpoint] = &handler.HandleCommandForm{}
		}
		if config.HTTPHandlers.GitlabBrowserEndpoint != "" {
			config.HTTPHandlers.GitlabBrowserEndpointConfig.MailClient = config.MailClient
			handlers[config.HTTPHandlers.GitlabBrowserEndpoint] = &config.HTTPHandlers.GitlabBrowserEndpointConfig
		}
		if config.HTTPHandlers.IndexEndpoints != nil {
			for _, location := range config.HTTPHandlers.IndexEndpoints {
				handlers[location] = &config.HTTPHandlers.IndexEndpointConfig
			}
		}
		if config.HTTPHandlers.MailMeEndpoint != "" {
			hand := config.HTTPHandlers.MailMeEndpointConfig
			hand.MailClient = config.MailClient
			handlers[config.HTTPHandlers.MailMeEndpoint] = &hand
		}
		// I (howard) personally need three bots, hence this ugly repetition.
		if config.HTTPHandlers.MicrosoftBotEndpoint1 != "" {
			hand := config.HTTPHandlers.MicrosoftBotEndpointConfig1
			handlers[config.HTTPHandlers.MicrosoftBotEndpoint1] = &hand
		}
		if config.HTTPHandlers.MicrosoftBotEndpoint2 != "" {
			hand := config.HTTPHandlers.MicrosoftBotEndpointConfig2
			handlers[config.HTTPHandlers.MicrosoftBotEndpoint2] = &hand
		}
		if config.HTTPHandlers.MicrosoftBotEndpoint3 != "" {
			hand := config.HTTPHandlers.MicrosoftBotEndpointConfig3
			handlers[config.HTTPHandlers.MicrosoftBotEndpoint3] = &hand
		}
		if proxyEndpoint := config.HTTPHandlers.WebProxyEndpoint; proxyEndpoint != "" {
			handlers[proxyEndpoint] = &handler.HandleWebProxy{OwnEndpoint: proxyEndpoint}
		}
		if config.HTTPHandlers.TwilioSMSEndpoint != "" {
			handlers[config.HTTPHandlers.TwilioSMSEndpoint] = &handler.HandleTwilioSMSHook{}
		}
		if config.HTTPHandlers.TwilioCallEndpoint != "" {
			/*
			 Configure a callback endpoint for Twilio call's callback.
			 The endpoint name is automatically generated from random bytes.
			*/
			randBytes := make([]byte, 32)
			_, err := rand.Read(randBytes)
			if err != nil {
				config.logger.Fatalf("GetHTTPD", "", err, "failed to read random number")
				return
			}
			callbackEndpoint := "/" + hex.EncodeToString(randBytes)
			// The greeting handler will use the callback endpoint to handle command
			config.HTTPHandlers.TwilioCallEndpointConfig.CallbackEndpoint = callbackEndpoint
			callEndpointConfig := config.HTTPHandlers.TwilioCallEndpointConfig
			callEndpointConfig.CallbackEndpoint = callbackEndpoint
			handlers[config.HTTPHandlers.TwilioCallEndpoint] = &callEndpointConfig
			// The callback handler will use the callback point that points to itself to carry on with phone conversation
			handlers[callbackEndpoint] = &handler.HandleTwilioCallCallback{MyEndpoint: callbackEndpoint}
		}
		config.HTTPDaemon.HandlerCollection = handlers
		if err := config.HTTPDaemon.Initialise(); err != nil {
			config.logger.Fatalf("GetHTTPD", "", err, "failed to initialise")
			return
		}
	})
	return config.HTTPDaemon
}

/*
Construct a mail command runner from configuration and return. It will use the common mail client to send replies.
The command runner is usually built into laitos' own SMTP daemon to process feature commands from incoming mails, but an
independent mail command runner is useful in certain scenarios, such as integrating with postfix's
"forward-mail-to-program" mechanism.
*/
func (config *Config) GetMailCommandRunner() *mailcmd.CommandRunner {
	config.mailCommandRunnerInit.Do(func() {
		// Assemble command processor from features and filters
		config.MailCommandRunner.Processor = &common.CommandProcessor{
			Features: config.Features,
			CommandFilters: []filter.CommandFilter{
				&config.MailFilters.PINAndShortcuts,
				&config.MailFilters.TranslateSequences,
			},
			ResultFilters: []filter.ResultFilter{
				&filter.ResetCombinedText{}, // this is mandatory but not configured by user's config file
				&config.MailFilters.LintText,
				&filter.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
				&config.MailFilters.NotifyViaEmail,
			},
		}
		config.MailCommandRunner.ReplyMailClient = config.MailClient
	})
	return config.MailCommandRunner
}

/*
Construct an SMTP daemon together with its mail command processor.
Both SMTP daemon and mail command processor will use the common mail client to forward mails and send replies.
*/
func (config *Config) GetMailDaemon() *smtpd.Daemon {
	config.mailDaemonInit.Do(func() {
		config.MailDaemon.CommandRunner = config.GetMailCommandRunner()
		config.MailDaemon.ForwardMailClient = config.MailClient
		if err := config.MailDaemon.Initialise(); err != nil {
			config.logger.Fatalf("GetMailDaemon", "", err, "failed to initialise")
			return
		}
	})
	return config.MailDaemon
}

/*
Construct a plain text protocol TCP&UDP daemon and return.
It will use common mail client for sending outgoing emails.
*/
func (config *Config) GetPlainSocketDaemon() *plainsocket.Daemon {
	config.plainSocketDaemonInit.Do(func() {
		// Assemble command processor from features and filters
		config.PlainSocketDaemon.Processor = &common.CommandProcessor{
			Features: config.Features,
			CommandFilters: []filter.CommandFilter{
				&config.PlainSocketFilters.PINAndShortcuts,
				&config.PlainSocketFilters.TranslateSequences,
			},
			ResultFilters: []filter.ResultFilter{
				&filter.ResetCombinedText{}, // this is mandatory but not configured by user's config file
				&config.PlainSocketFilters.LintText,
				&filter.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
				&config.PlainSocketFilters.NotifyViaEmail,
			},
		}
		// Call initialise so that daemon is ready to start
		if err := config.PlainSocketDaemon.Initialise(); err != nil {
			config.logger.Fatalf("GetPlainSocketDaemon", "", err, "failed to initialise")
			return
		}
	})
	return config.PlainSocketDaemon
}

// Intentionally undocumented
func (config *Config) GetSockDaemon() *sockd.Daemon {
	config.sockDaemonInit.Do(func() {
		config.SockDaemon.DNSDaemon = config.GetDNSD()
		if err := config.SockDaemon.Initialise(); err != nil {
			config.logger.Fatalf("GetSockDaemon", "", err, "failed to initialise")
			return
		}
	})
	return config.SockDaemon
}

// Construct a telegram bot from configuration and return.
func (config *Config) GetTelegramBot() *telegrambot.Daemon {
	config.telegramBotInit.Do(func() {
		// Assemble telegram bot from features and filters
		config.TelegramBot.Processor = &common.CommandProcessor{
			Features: config.Features,
			CommandFilters: []filter.CommandFilter{
				&config.TelegramFilters.PINAndShortcuts,
				&config.TelegramFilters.TranslateSequences,
			},
			ResultFilters: []filter.ResultFilter{
				&filter.ResetCombinedText{}, // this is mandatory but not configured by user's config file
				&config.TelegramFilters.LintText,
				&filter.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
				&config.TelegramFilters.NotifyViaEmail,
			},
		}
		if err := config.TelegramBot.Initialise(); err != nil {
			config.logger.Fatalf("GetTelegramBot", "", err, "failed to initialise")
			return
		}
	})
	return config.TelegramBot
}
