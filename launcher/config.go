package launcher

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"

	"github.com/HouzuoGuo/laitos/awsinteg"
	"github.com/HouzuoGuo/laitos/daemon/httpproxy"
	"github.com/HouzuoGuo/laitos/daemon/passwdrpc"
	"github.com/HouzuoGuo/laitos/daemon/phonehome"
	"github.com/HouzuoGuo/laitos/misc"

	"github.com/HouzuoGuo/laitos/daemon/autounlock"
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/daemon/httpd"
	"github.com/HouzuoGuo/laitos/daemon/httpd/handler"
	"github.com/HouzuoGuo/laitos/daemon/maintenance"
	"github.com/HouzuoGuo/laitos/daemon/plainsocket"
	"github.com/HouzuoGuo/laitos/daemon/simpleipsvcd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/daemon/snmpd"
	"github.com/HouzuoGuo/laitos/daemon/sockd"
	"github.com/HouzuoGuo/laitos/daemon/telegrambot"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const (

	/*
		EnvironmentStripURLPrefixFromRequest is an environemnt variable name, the value of which is an optional prefix string that is expected
		to be present in request URLs.
		If used, HTTP server will install all of its handlers at URL location according to the server configuration, but with the prefix
		URL string added to each of them.
		This often helps when some kind of API gateway (e.g. AWS API gateway) proxies visitors' requests and places a prefix string in
		each request.
		For example: a homepage's domain is served by a CDN, the CDN forwards visitors' requests to a backend ("origin") and in doing
		so automatically adds a URL prefix "/stageLive" because the backend expects such prefix. In this case, the stripURLPrefixFromRequest
		shall be "/stageLive".
	*/
	EnvironmentStripURLPrefixFromRequest = "LAITOS_STRIP_URL_PREFIX_FROM_REQUEST"

	/*
		   EnvironmentStripURLPrefixFromResponse is an environment variable name, the value of which is is an optional prefix string that will
			 be stirpped from rendered HTML pages, such as links on pages and form action URLs, this is usually used in conjunction with
			 EnvironmentStripURLPrefixFromRequest.
	*/
	EnvironmentStripURLPrefixFromResponse = "LAITOS_STRIP_URL_PREFIX_FROM_RESPONSE"
)

/*
StandardFilters contains a standard set of filters (PIN match, notification, lint, etc) that are useful to nearly all
laitos daemons that carry a command processor. The filters' configuration is fed by Config struct, which is itself fed
by deserialised JSON text.
*/
type StandardFilters struct {
	// For input command content
	TranslateSequences toolbox.TranslateSequences `json:"TranslateSequences"`
	PINAndShortcuts    toolbox.PINAndShortcuts    `json:"PINAndShortcuts"`

	// For command execution result
	NotifyViaEmail toolbox.NotifyViaEmail `json:"NotifyViaEmail"`
	LintText       toolbox.LintText       `json:"LintText"`
}

// Configure path to HTTP handlers and handler themselves.
type HTTPHandlers struct {
	AppCommandEndpoint              string                          `json:"AppCommandEndpoint"`
	CommandFormEndpoint             string                          `json:"CommandFormEndpoint"`
	FileUploadEndpoint              string                          `json:"FileUploadEndpoint"`
	GitlabBrowserEndpoint           string                          `json:"GitlabBrowserEndpoint"`
	GitlabBrowserEndpointConfig     handler.HandleGitlabBrowser     `json:"GitlabBrowserEndpointConfig"`
	IndexEndpointConfig             handler.HandleHTMLDocument      `json:"IndexEndpointConfig"`
	IndexEndpoints                  []string                        `json:"IndexEndpoints"`
	InformationEndpoint             string                          `json:"InformationEndpoint"`
	LatestRequestsInspectorEndpoint string                          `json:"LatestRequestsInspectorEndpoint"`
	MailMeEndpoint                  string                          `json:"MailMeEndpoint"`
	MailMeEndpointConfig            handler.HandleMailMe            `json:"MailMeEndpointConfig"`
	MessageBankEndpoint             string                          `json:"MessageBankEndpoint"`
	MicrosoftBotEndpoint1           string                          `json:"MicrosoftBotEndpoint1"`
	MicrosoftBotEndpoint2           string                          `json:"MicrosoftBotEndpoint2"`
	MicrosoftBotEndpoint3           string                          `json:"MicrosoftBotEndpoint3"`
	MicrosoftBotEndpointConfig1     handler.HandleMicrosoftBot      `json:"MicrosoftBotEndpointConfig1"`
	MicrosoftBotEndpointConfig2     handler.HandleMicrosoftBot      `json:"MicrosoftBotEndpointConfig2"`
	MicrosoftBotEndpointConfig3     handler.HandleMicrosoftBot      `json:"MicrosoftBotEndpointConfig3"`
	ProcessExplorerEndpoint         string                          `json:"ProcessExplorerEndpoint"`
	PrometheusMetricsEndpoint       string                          `json:"PrometheusMetricsEndpoint"`
	RecurringCommandsEndpoint       string                          `json:"RecurringCommandsEndpoint"`
	RecurringCommandsEndpointConfig handler.HandleRecurringCommands `json:"RecurringCommandsEndpointConfig"`
	ReportsRetrievalEndpoint        string                          `json:"ReportsRetrievalEndpoint"`
	RequestInspectorEndpoint        string                          `json:"RequestInspectorEndpoint"`
	LoraWANWebhookEndpoint          string                          `json:"LoraWANWebhookEndpoint"`
	TwilioCallEndpoint              string                          `json:"TwilioCallEndpoint"`
	TwilioCallEndpointConfig        handler.HandleTwilioCallHook    `json:"TwilioCallEndpointConfig"`
	TwilioSMSEndpoint               string                          `json:"TwilioSMSEndpoint"`
	VirtualMachineEndpoint          string                          `json:"VirtualMachineEndpoint"`
	VirtualMachineEndpointConfig    handler.HandleVirtualMachine    `json:"VirtualMachineEndpointConfig"`
	WebProxyEndpoint                string                          `json:"WebProxyEndpoint"`
}

// AWSIntegration contains configuration properties for global behaviours (e.g. logger) of laitos program to integrate with AWS
// services such as SQS.
type AWSIntegration struct {
	// SendWarningLogToSQSURL is the URL of SQS queue that will receive a copy of each warning log entry.
	// The queue will however ignore the warning log entries generated repeatedly by the same actor.
	SendWarningLogToSQSURL string `json:"SendWarningLogToSQSURL"`
	// ForwardMessageProcessorReportsToFirehoseStreamName is the name of kinesis firestream that will receive a copy of each
	// subject report as they arrive.
	ForwardMessageProcessorReportsToFirehoseStreamName string `json:"ForwardMessageProcessorReportsToFirehoseStreamName"`
	// ForwardMessageProcessorReportsToSNSTopicARN is the ARN of SNS topic that will receive a copy of each subject report as they
	// arrive.
	ForwardMessageProcessorReportsToSNSTopicARN string `json:"ForwardMessageProcessorReportsToSNSTopicARN"`
}

// Config is an aggregated structure of configuration properties that include daemon settings, mail settings, cloud integration
// settings, app settings, and so on.
// The entry point of laitos program deserialises this structure from a (often) hand-crafted configuration file written in JSON.
type Config struct {
	/*
		Features consist of all app instances, shared by all daemons and command runners. Avoid duplicating this
		structure because certain app features (such as AES file decryption) may hold large amount of data in
		memory. Therefore, all daemons with app command execution capability share the same app instances.
	*/
	Features *toolbox.FeatureSet `json:"Features"`

	// MessageProcessorFilters configure the Message Processor app's own command processor.
	MessageProcessorFilters StandardFilters `json:"MessageProcessorFilters"`

	MailClient inet.MailClient `json:"MailClient"` // MailClient is the common client configuration for sending notification emails and mail command runner results.

	Maintenance *maintenance.Daemon `json:"Maintenance"` // Daemon configures behaviour of periodic health-check/system maintenance

	DNSDaemon  *dnsd.Daemon    `json:"DNSDaemon"`  // DNSDaemon: configure DNS daemon's network behaviour
	DNSFilters StandardFilters `json:"DNSFilters"` // DNSFilters: configure DNS daemon's toolbox command processor

	HTTPDaemon   *httpd.Daemon   `json:"HTTPDaemon"`   // HTTP daemon configuration
	HTTPFilters  StandardFilters `json:"HTTPFilters"`  // HTTP daemon filter configuration
	HTTPHandlers HTTPHandlers    `json:"HTTPHandlers"` // HTTP daemon handler configuration

	MailDaemon        *smtpd.Daemon          `json:"MailDaemon"`        // SMTP daemon configuration
	MailCommandRunner *mailcmd.CommandRunner `json:"MailCommandRunner"` // MailCommandRunner processes toolbox commands from incoming mail body.

	MailFilters StandardFilters `json:"MailFilters"` // MailFilters configure command processor for mail command runner

	PhoneHomeDaemon  *phonehome.Daemon `json:"PhoneHomeDaemon"`  // PhoneHomeDaemon daemon instance and daemon configuration
	PhoneHomeFilters StandardFilters   `json:"PhoneHomeFilters"` // PhoneHomeFilters daemon command processor configuration

	PlainSocketDaemon  *plainsocket.Daemon `json:"PlainSocketDaemon"`  // Plain text protocol TCP and UDP daemon configuration
	PlainSocketFilters StandardFilters     `json:"PlainSocketFilters"` // Plain text daemon filter configuration

	SockDaemon *sockd.Daemon `json:"SockDaemon"` // Intentionally undocumented

	SNMPDaemon *snmpd.Daemon `json:"SNMPDaemon"` // SNMPDaemon configuration and instance

	SimpleIPSvcDaemon *simpleipsvcd.Daemon `json:"SimpleIPSvcDaemon"` // SimpleIPSvcDaemon is the simple TCP/UDP service daemon configuration and instance

	TelegramBot     *telegrambot.Daemon `json:"TelegramBot"`     // Telegram bot configuration
	TelegramFilters StandardFilters     `json:"TelegramFilters"` // Telegram bot filter configuration

	AutoUnlock *autounlock.Daemon `json:"AutoUnlock"` // AutoUnlock daemon
	// PasswordRPCDaemon offers a network listener for a gRPC service that allows other laitos program instances to obtain password for unlocking their encrypted config/data files.
	PasswordRPCDaemon *passwdrpc.Daemon `json:"PasswordRPCDaemon"`
	// HTTPProxyDaemon offers an HTTP proxy capable of handling both HTTP and HTTPS destinations.
	HTTPProxyDaemon *httpproxy.Daemon `json:"HTTPProxyDaemon"`

	SupervisorNotificationRecipients []string `json:"SupervisorNotificationRecipients"` // Email addresses of supervisor notification recipients

	// AWSIntegration are settings for integrating with various AWS services, such as S3 and SQS.
	AWSIntegration AWSIntegration `json:"AWSIntegration"`

	logger                *lalog.Logger // logger handles log output from configuration serialisation and initialisation routines.
	maintenanceInit       *sync.Once
	dnsDaemonInit         *sync.Once
	snmpDaemonInit        *sync.Once
	simpleIPSvcDaemonInit *sync.Once
	httpDaemonInit        *sync.Once
	mailCommandRunnerInit *sync.Once
	mailDaemonInit        *sync.Once
	phoneHomeDaemonInit   *sync.Once
	plainSocketDaemonInit *sync.Once
	sockDaemonInit        *sync.Once
	telegramBotInit       *sync.Once
	autoUnlockInit        *sync.Once
	passwdrpcDaemonInit   *sync.Once
	httpProxyDaemonInit   *sync.Once
}

// Initialise decorates feature configuration and command bridge configuration in preparation for daemon operations.
func (config *Config) Initialise() error {
	// An empty FeatureSet can still offer several useful features such as program environment control and running shell commands
	if config.Features == nil {
		config.Features = &toolbox.FeatureSet{}
	}

	// Initialise the optional AWS kinesis firehose client for a stream to get a copy of every report received by message processor
	var firehoseClient *awsinteg.KinesisHoseClient
	var err error
	if streamName := config.AWSIntegration.ForwardMessageProcessorReportsToFirehoseStreamName; streamName != "" && misc.EnableAWSIntegration {
		config.logger.Info("", nil, "initialising kinesis firehose client for stream \"%s\"", streamName)
		firehoseClient, err = awsinteg.NewKinesisHoseClient()
		if err != nil {
			config.logger.Warning("", err, "failed to initialise kinesis firehose client")
		}
	}
	// Initialise the optional AWS SNS client for a topic to get a copy of every report received by message processor
	var snsClient *awsinteg.SNSClient
	if arn := config.AWSIntegration.ForwardMessageProcessorReportsToSNSTopicARN; arn != "" && misc.EnableAWSIntegration {
		config.logger.Info("", nil, "initialising SNS client for topic ARN \"%s\"", arn)
		snsClient, err = awsinteg.NewSNSClient()
		if err != nil {
			config.logger.Warning("", err, "failed to initialise SNS client")
		}
	}

	/*
		Even though MessageProcessor is an app, it has its own command processor just like a daemon.
		The command processor is initialised from configuration input.
	*/
	if len(config.MessageProcessorFilters.PINAndShortcuts.Passwords) != 0 {
		messageProcessorCommandProcessor := &toolbox.CommandProcessor{
			Features: config.Features,
			CommandFilters: []toolbox.CommandFilter{
				&config.MessageProcessorFilters.PINAndShortcuts,
				&config.MessageProcessorFilters.TranslateSequences,
			},
			ResultFilters: []toolbox.ResultFilter{
				&config.MessageProcessorFilters.LintText,
				&toolbox.SayEmptyOutput{},
				&config.MessageProcessorFilters.NotifyViaEmail,
			},
		}
		config.Features.MessageProcessor = toolbox.MessageProcessor{
			OwnerName:                       "app",
			CmdProcessor:                    messageProcessorCommandProcessor,
			ForwardReportsToKinesisFirehose: firehoseClient,
			KinesisFirehoseStreamName:       config.AWSIntegration.ForwardMessageProcessorReportsToFirehoseStreamName,
			ForwardReportsToSNS:             snsClient,
			SNSTopicARN:                     config.AWSIntegration.ForwardMessageProcessorReportsToSNSTopicARN,
		}
	}
	/*
		Fill in some blanks so that Get*Daemon functions will be able to call Initialise() function at very least.
		So that if a user turns on a daemon in the daemon list but forgets to write its configuration, the individual daemon will
		try to initialise itself (otherwise it's a nil pointer panic), and reports a helpful initialisation error reminding the
		user of the missing configuration.
	*/
	config.mailCommandRunnerInit = new(sync.Once)
	if config.MailCommandRunner == nil {
		config.MailCommandRunner = &mailcmd.CommandRunner{}
	}
	config.maintenanceInit = new(sync.Once)
	if config.Maintenance == nil {
		config.Maintenance = &maintenance.Daemon{}
	}
	config.dnsDaemonInit = new(sync.Once)
	if config.DNSDaemon == nil {
		config.DNSDaemon = &dnsd.Daemon{}
	}
	config.httpDaemonInit = new(sync.Once)
	if config.HTTPDaemon == nil {
		config.HTTPDaemon = &httpd.Daemon{}
	}
	config.mailDaemonInit = new(sync.Once)
	if config.MailDaemon == nil {
		config.MailDaemon = &smtpd.Daemon{}
	}
	config.phoneHomeDaemonInit = new(sync.Once)
	if config.PhoneHomeDaemon == nil {
		config.PhoneHomeDaemon = &phonehome.Daemon{}
	}
	config.plainSocketDaemonInit = new(sync.Once)
	if config.PlainSocketDaemon == nil {
		config.PlainSocketDaemon = &plainsocket.Daemon{}
	}
	config.simpleIPSvcDaemonInit = new(sync.Once)
	if config.SimpleIPSvcDaemon == nil {
		config.SimpleIPSvcDaemon = &simpleipsvcd.Daemon{}
	}
	config.snmpDaemonInit = new(sync.Once)
	if config.SNMPDaemon == nil {
		config.SNMPDaemon = &snmpd.Daemon{}
	}
	config.sockDaemonInit = new(sync.Once)
	if config.SockDaemon == nil {
		config.SockDaemon = &sockd.Daemon{}
	}
	config.telegramBotInit = new(sync.Once)
	if config.TelegramBot == nil {
		config.TelegramBot = &telegrambot.Daemon{}
	}
	config.autoUnlockInit = new(sync.Once)
	if config.AutoUnlock == nil {
		config.AutoUnlock = &autounlock.Daemon{}
	}
	config.passwdrpcDaemonInit = new(sync.Once)
	if config.PasswordRPCDaemon == nil {
		config.PasswordRPCDaemon = &passwdrpc.Daemon{}
	}
	config.httpProxyDaemonInit = new(sync.Once)
	if config.HTTPProxyDaemon == nil {
		config.HTTPProxyDaemon = &httpproxy.Daemon{}
	}
	// All notification filters share the common mail client
	config.MessageProcessorFilters.NotifyViaEmail.MailClient = config.MailClient
	config.DNSFilters.NotifyViaEmail.MailClient = config.MailClient
	config.HTTPFilters.NotifyViaEmail.MailClient = config.MailClient
	config.MailFilters.NotifyViaEmail.MailClient = config.MailClient
	config.PhoneHomeFilters.NotifyViaEmail.MailClient = config.MailClient
	config.PlainSocketFilters.NotifyViaEmail.MailClient = config.MailClient
	config.TelegramFilters.NotifyViaEmail.MailClient = config.MailClient
	// SendMail feature also shares the common mail client
	config.Features.SendMail.MailClient = config.MailClient
	if err := config.Features.Initialise(); err != nil {
		return err
	}
	// Password RPC daemon shares the embedded gRPC service with the network bound file encryption app
	config.PasswordRPCDaemon.PasswordRegister = config.Features.NetBoundFileEncryption.PasswordRegister

	config.logger.Info("", nil, "enabled features are - %v", config.Features.GetTriggers())
	return nil
}

/*
DeserialiseFromJSON deserialised configuration of all daemons and toolbox features from JSON input, and then prepares
itself for daemon operations.
*/
func (config *Config) DeserialiseFromJSON(in []byte) error {
	config.logger = &lalog.Logger{ComponentName: "config"}
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
		// Assemble DNS command prcessor from features and filters
		config.DNSDaemon.Processor = &toolbox.CommandProcessor{
			Features: config.Features,
			CommandFilters: []toolbox.CommandFilter{
				&config.DNSFilters.PINAndShortcuts,
				&config.DNSFilters.TranslateSequences,
			},
			ResultFilters: []toolbox.ResultFilter{
				&config.DNSFilters.LintText,
				&toolbox.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
				&config.DNSFilters.NotifyViaEmail,
			},
		}
		if err := config.DNSDaemon.Initialise(); err != nil {
			config.logger.Abort("", err, "the daemon failed to initialise")
			return
		}
	})
	return config.DNSDaemon
}

// GetSNMPD initialises SNMP daemon instance and returns it.
func (config *Config) GetSNMPD() *snmpd.Daemon {
	config.snmpDaemonInit.Do(func() {
		if err := config.SNMPDaemon.Initialise(); err != nil {
			config.logger.Abort("", err, "the daemon failed to initialise")
			return
		}
	})
	return config.SNMPDaemon
}

// GetSimpleIPSvcD initialises simple IP services daemon and returns it.
func (config *Config) GetSimpleIPSvcD() *simpleipsvcd.Daemon {
	config.simpleIPSvcDaemonInit.Do(func() {
		if err := config.SimpleIPSvcDaemon.Initialise(); err != nil {
			config.logger.Abort("", err, "the daemon failed to initialise")
			return
		}
	})
	return config.SimpleIPSvcDaemon
}

// GetMaintenance constructs a system maintenance / health check daemon from configuration and return.
func (config *Config) GetMaintenance() *maintenance.Daemon {
	config.maintenanceInit.Do(func() {
		config.Maintenance.ToolboxSelfTest = config.Features
		config.Maintenance.MailClient = config.MailClient
		config.Maintenance.MailCommandRunnerSelfTest = config.GetMailCommandRunner()
		config.Maintenance.HttpHandlersSelfTest = config.GetHTTPD().HandlerCollection
		if err := config.Maintenance.Initialise(); err != nil {
			config.logger.Abort("", err, "the daemon failed to initialise")
			return
		}
	})
	return config.Maintenance
}

// Construct an HTTP daemon from configuration and return.
func (config *Config) GetHTTPD() *httpd.Daemon {
	config.httpDaemonInit.Do(func() {
		// Assemble command processor from features and filters
		config.HTTPDaemon.Processor = &toolbox.CommandProcessor{
			Features: config.Features,
			CommandFilters: []toolbox.CommandFilter{
				&config.HTTPFilters.PINAndShortcuts,
				&config.HTTPFilters.TranslateSequences,
			},
			ResultFilters: []toolbox.ResultFilter{
				&config.HTTPFilters.LintText,
				&toolbox.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
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
		// Configure a virtual machine screenshot endpoint at a randomly generated endpoint name
		if config.HTTPHandlers.VirtualMachineEndpoint != "" {
			randBytes := make([]byte, 32)
			_, err := rand.Read(randBytes)
			if err != nil {
				config.logger.Abort("", err, "failed to read random number")
				return
			}
			// The screenshot endpoint
			vmScreenshotHandler := &handler.HandleVirtualMachineScreenshot{}
			vmHandler := config.HTTPHandlers.VirtualMachineEndpointConfig
			screenshotEndpoint := "/vm-screenshot-" + hex.EncodeToString(randBytes)
			handlers[screenshotEndpoint] = vmScreenshotHandler
			// The VM control endpoint is given the screenshot endpoint location and instance
			vmHandler.ScreenshotEndpoint = screenshotEndpoint
			vmHandler.ScreenshotHandlerInstance = vmScreenshotHandler
			handlers[config.HTTPHandlers.VirtualMachineEndpoint] = &vmHandler
		}

		if config.HTTPHandlers.CommandFormEndpoint != "" {
			handlers[config.HTTPHandlers.CommandFormEndpoint] = &handler.HandleCommandForm{}
		}
		if config.HTTPHandlers.FileUploadEndpoint != "" {
			handlers[config.HTTPHandlers.FileUploadEndpoint] = &handler.HandleFileUpload{}
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
		if config.HTTPHandlers.RecurringCommandsEndpoint != "" {
			handlers[config.HTTPHandlers.RecurringCommandsEndpoint] = &config.HTTPHandlers.RecurringCommandsEndpointConfig
		}
		if proxyEndpoint := config.HTTPHandlers.WebProxyEndpoint; proxyEndpoint != "" {
			handlers[proxyEndpoint] = &handler.HandleWebProxy{OwnEndpoint: proxyEndpoint}
		}
		if endpoint := config.HTTPHandlers.LoraWANWebhookEndpoint; endpoint != "" {
			handlers[endpoint] = &handler.HandleLoraWANWebhook{}
		}
		if config.HTTPHandlers.MessageBankEndpoint != "" {
			handlers[config.HTTPHandlers.MessageBankEndpoint] = &handler.HandleMessageBank{}
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
				config.logger.Abort("", err, "failed to read random number")
				return
			}
			callbackEndpoint := "/twilio-callback-" + hex.EncodeToString(randBytes)
			// The greeting handler will use the callback endpoint to handle command
			config.HTTPHandlers.TwilioCallEndpointConfig.CallbackEndpoint = callbackEndpoint
			callEndpointConfig := config.HTTPHandlers.TwilioCallEndpointConfig
			callEndpointConfig.CallbackEndpoint = callbackEndpoint
			handlers[config.HTTPHandlers.TwilioCallEndpoint] = &callEndpointConfig
			// The callback handler will use the callback point that points to itself to carry on with phone conversation
			handlers[callbackEndpoint] = &handler.HandleTwilioCallCallback{MyEndpoint: callbackEndpoint}
		}
		if config.HTTPHandlers.AppCommandEndpoint != "" {
			handlers[config.HTTPHandlers.AppCommandEndpoint] = &handler.HandleAppCommand{}
		}
		if config.HTTPHandlers.ReportsRetrievalEndpoint != "" {
			handlers[config.HTTPHandlers.ReportsRetrievalEndpoint] = &handler.HandleReportsRetrieval{}
		}
		if config.HTTPHandlers.ProcessExplorerEndpoint != "" {
			handlers[config.HTTPHandlers.ProcessExplorerEndpoint] = &handler.HandleProcessExplorer{}
		}
		if config.HTTPHandlers.PrometheusMetricsEndpoint != "" {
			handlers[config.HTTPHandlers.PrometheusMetricsEndpoint] = &handler.HandlePrometheus{}
		}
		if config.HTTPHandlers.RequestInspectorEndpoint != "" {
			handlers[config.HTTPHandlers.RequestInspectorEndpoint] = &handler.HandleRequestInspector{}
		}
		if config.HTTPHandlers.LatestRequestsInspectorEndpoint != "" {
			handlers[config.HTTPHandlers.LatestRequestsInspectorEndpoint] = &handler.HandleLatestRequestsInspector{}
		}
		config.HTTPDaemon.HandlerCollection = handlers
		stripURLPrefixFromRequest := os.Getenv(EnvironmentStripURLPrefixFromRequest)
		stripURLPrefixFromResponse := os.Getenv(EnvironmentStripURLPrefixFromResponse)
		config.logger.Info("", nil, "will strip \"%s\" from requested URLs and strip \"%s\" from HTML response", stripURLPrefixFromRequest, stripURLPrefixFromResponse)
		if err := config.HTTPDaemon.Initialise(stripURLPrefixFromRequest, stripURLPrefixFromResponse); err != nil {
			config.logger.Abort("", err, "the daemon failed to initialise")
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
		config.MailCommandRunner.Processor = &toolbox.CommandProcessor{
			Features: config.Features,
			CommandFilters: []toolbox.CommandFilter{
				&config.MailFilters.PINAndShortcuts,
				&config.MailFilters.TranslateSequences,
			},
			ResultFilters: []toolbox.ResultFilter{
				&config.MailFilters.LintText,
				&toolbox.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
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
			config.logger.Abort("", err, "the daemon failed to initialise")
			return
		}
	})
	return config.MailDaemon
}

// GetPhoneHomeDaemon initialises a Phone-Home daemon and returns it.
func (config *Config) GetPhoneHomeDaemon() *phonehome.Daemon {
	config.phoneHomeDaemonInit.Do(func() {
		config.PhoneHomeDaemon.Processor = &toolbox.CommandProcessor{
			Features: config.Features,
			CommandFilters: []toolbox.CommandFilter{
				&config.PhoneHomeFilters.PINAndShortcuts,
				&config.PhoneHomeFilters.TranslateSequences,
			},
			ResultFilters: []toolbox.ResultFilter{
				&config.PhoneHomeFilters.LintText,
				&toolbox.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
				&config.PhoneHomeFilters.NotifyViaEmail,
			},
		}
		// Call initialise so that daemon is ready to start
		if err := config.PhoneHomeDaemon.Initialise(); err != nil {
			config.logger.Abort("", err, "the daemon failed to initialise")
			return
		}
	})
	return config.PhoneHomeDaemon
}

/*
Construct a plain text protocol TCP&UDP daemon and return.
It will use common mail client for sending outgoing emails.
*/
func (config *Config) GetPlainSocketDaemon() *plainsocket.Daemon {
	config.plainSocketDaemonInit.Do(func() {
		// Assemble command processor from features and filters
		config.PlainSocketDaemon.Processor = &toolbox.CommandProcessor{
			Features: config.Features,
			CommandFilters: []toolbox.CommandFilter{
				&config.PlainSocketFilters.PINAndShortcuts,
				&config.PlainSocketFilters.TranslateSequences,
			},
			ResultFilters: []toolbox.ResultFilter{
				&config.PlainSocketFilters.LintText,
				&toolbox.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
				&config.PlainSocketFilters.NotifyViaEmail,
			},
		}
		// Call initialise so that daemon is ready to start
		if err := config.PlainSocketDaemon.Initialise(); err != nil {
			config.logger.Abort("", err, "the daemon failed to initialise")
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
			config.logger.Abort("", err, "the daemon failed to initialise")
			return
		}
	})
	return config.SockDaemon
}

// Construct a telegram bot from configuration and return.
func (config *Config) GetTelegramBot() *telegrambot.Daemon {
	config.telegramBotInit.Do(func() {
		// Assemble telegram bot from features and filters
		config.TelegramBot.Processor = &toolbox.CommandProcessor{
			Features: config.Features,
			CommandFilters: []toolbox.CommandFilter{
				&config.TelegramFilters.PINAndShortcuts,
				&config.TelegramFilters.TranslateSequences,
			},
			ResultFilters: []toolbox.ResultFilter{
				&config.TelegramFilters.LintText,
				&toolbox.SayEmptyOutput{}, // this is mandatory but not configured by user's config file
				&config.TelegramFilters.NotifyViaEmail,
			},
		}
		if err := config.TelegramBot.Initialise(); err != nil {
			config.logger.Abort("", err, "the daemon failed to initialise")
			return
		}
	})
	return config.TelegramBot
}

// GetAutoUnlock constructs the auto-unlock prober and returns.
func (config *Config) GetAutoUnlock() *autounlock.Daemon {
	config.autoUnlockInit.Do(func() {
		if err := config.AutoUnlock.Initialise(); err != nil {
			config.logger.Abort("", err, "the daemon failed to initialise")
			return
		}
	})
	return config.AutoUnlock
}

// GetPasswdRPCDaemon returns the network daemon that allows other laitos program instances to obtain password
// for unlocking their encrypted config/data files.
func (config *Config) GetPasswdRPCDaemon() *passwdrpc.Daemon {
	config.passwdrpcDaemonInit.Do(func() {
		if err := config.PasswordRPCDaemon.Initialise(); err != nil {
			config.logger.Abort("", err, "the daemon failed to initialise")
			return
		}
	})
	return config.PasswordRPCDaemon
}

// GetHTTPProxyDaemon returns an initialised instance of HTTP proxy daemon.
func (config *Config) GetHTTPProxyDaemon() *httpproxy.Daemon {
	config.httpProxyDaemonInit.Do(func() {
		config.HTTPProxyDaemon.CommandProcessor = toolbox.GetEmptyCommandProcessor()
		config.HTTPProxyDaemon.CommandProcessor.Features = config.Features
		config.HTTPProxyDaemon.DNSDaemon = config.GetDNSD()
		if err := config.HTTPProxyDaemon.Initialise(); err != nil {
			config.logger.Abort("", err, "the daemon failed to initialise")
			return
		}
	})
	return config.HTTPProxyDaemon
}
