package main

// A single configuration file format dictates all functions of this program.
type Config struct {
	MessageEndpoint     string // The secret API endpoint name for messaging in daemon mode
	VoiceMLEndpoint     string // The secret API endpoint name that serves TwiML voice script in daemon mode
	VoiceProcEndpoint   string // The secret API endpoint name that responds to TwiML voice script
	VoiceEndpointPrefix string // The HTTP scheme and/or host name and/or URL prefix that will correctly construct URLs leading to ML and Proc endpoints
	ServerPort          int    // The port HTTP server listens on in daemon mode
	PIN                 string // The pre-shared secret pin to enable command execution in both daemon and mail mode
	TLSCert             string // Location of HTTP TLS certificate in daemon mode
	TLSKey              string // Location of HTTP TLS key in daemon mode

	SubHashSlashForPipe bool // Substitute char sequence #/ from incoming command for char | before command execution
	WebTimeoutSec       int  // When reached from web API, WolframAlpha query/shell command is killed after this number of seconds.
	WebTruncateLen      int  // When reached from web API, truncate command execution result to this length.

	MailTimeoutSec       int      // When reached from mail API, WolframAlpha query/shell command is killed after this number of seconds.
	MailTruncateLen      int      // When reached from mail API, truncate command execution result to this length.
	MailRecipients       []string // List of mail addresses that receive notification after each command
	MailFrom             string   // FROM address of the mail notifications
	MailAgentAddressPort string   // Address and port number of mail transportation agent for sending notifications

	MysteriousURL         string   // intentionally undocumented
	MysteriousAddr1       string   // intentionally undocumented
	MysteriousAddr2       string   // intentionally undocumented
	MysteriousID1         string   // intentionally undocumented
	MysteriousID2         string   // intentionally undocumented
	MysteriousCmds        []string // intentionally undocumented
	MysteriousCmdIntvHour int      // intentionally undocumented

	TwilioNumber     string // Twilio telephone number for outbound call and SMS
	TwilioSID        string // Twilio account SID
	TwilioAuthSecret string // Twilio authentication secret token

	WolframAlphaAppID string // WolframAlpha application ID for consuming its APIs

	PresetMessages map[string]string // Pre-defined mapping of secret phrases and their  corresponding command
}

// Return a web API server instance with completed configuration.
func (conf *Config) ToWebServer() APIServer {
	var cmdRunner CommandRunner
	cmdRunner = CommandRunner{
		SubHashSlashForPipe: conf.SubHashSlashForPipe,
		SqueezeIntoOneLine:  true,
		TimeoutSec:          conf.WebTimeoutSec,
		TruncateLen:         conf.WebTruncateLen,
		PIN:                 conf.PIN,
		PresetMessages:      conf.PresetMessages,
		Mailer: Mailer{
			MailFrom:       conf.MailFrom,
			MTAAddressPort: conf.MailAgentAddressPort,
			Recipients:     conf.MailRecipients,
		},
		Twilio: TwilioClient{
			AccountSID:  conf.TwilioSID,
			AuthSecret:  conf.TwilioAuthSecret,
			PhoneNumber: conf.TwilioNumber},
		WolframAlpha: WolframAlphaClient{AppID: conf.WolframAlphaAppID},
	}
	return APIServer{
		MessageEndpoint:     conf.MessageEndpoint,
		VoiceMLEndpoint:     conf.VoiceMLEndpoint,
		VoiceProcEndpoint:   conf.VoiceProcEndpoint,
		VoiceEndpointPrefix: conf.VoiceEndpointPrefix,
		ServerPort:          conf.ServerPort,
		TLSCert:             conf.TLSCert,
		TLSKey:              conf.TLSKey,
		Command:             cmdRunner,
	}
}

// Return a mail processor instance with completed configuration.
func (conf *Config) ToMailProcessor() MailProcessor {
	var cmdRunner CommandRunner
	cmdRunner = CommandRunner{
		SubHashSlashForPipe: conf.SubHashSlashForPipe,
		SqueezeIntoOneLine:  true,
		TimeoutSec:          conf.MailTimeoutSec,
		TruncateLen:         conf.MailTruncateLen,
		PIN:                 conf.PIN,
		PresetMessages:      conf.PresetMessages,
		Mailer: Mailer{
			MailFrom:       conf.MailFrom,
			MTAAddressPort: conf.MailAgentAddressPort,
			Recipients:     conf.MailRecipients,
		},
		Twilio: TwilioClient{
			AccountSID:  conf.TwilioSID,
			AuthSecret:  conf.TwilioAuthSecret,
			PhoneNumber: conf.TwilioNumber},
		WolframAlpha: WolframAlphaClient{AppID: conf.WolframAlphaAppID},
	}
	return MailProcessor{
		CommandRunner: cmdRunner,
		Mysterious: MysteriousClient{
			Addr1:           conf.MysteriousAddr1,
			Addr2:           conf.MysteriousAddr2,
			CmdIntervalHour: conf.MysteriousCmdIntvHour,
			Cmds:            conf.MysteriousCmds,
			ID1:             conf.MysteriousID1,
			ID2:             conf.MysteriousID2,
			URL:             conf.MysteriousURL,
		},
	}
}
