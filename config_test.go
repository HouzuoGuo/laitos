package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestConfig(t *testing.T) {
	confText := `{
    "MessageEndpoint": "my_secret_endpoint_name_without_leading_slash",
    "VoiceMLEndpoint": "twilio_hook_initial_contact_without_leading_slash",
    "VoiceProcEndpoint": "twilio_hook_processor_without_leading_slash",
    "VoiceEndpointPrefix": "/optional_voice_hook_proxy_prefix",
    "ServerPort": 12321,
    "PIN": "MYSECRET",
    "TLSCert": "/tmp/test.crt",
    "TLSKey": "/tmp/test.key",

    "SubHashSlashForPipe": true,
    "WebTimeoutSec": 10,
    "WebTruncateLen": 120,

    "MailTimeoutSec": 20,
    "MailTruncateLen": 240,
    "MailRecipients": ["ITsupport@mydomain.com"],
    "MailFrom": "admin@mydomain.com",
    "MailAgentAddressPort": "mydomain.com:25",

    "PresetMessages": {
        "secretapple": "echo hello world",
        "secretpineapple": "poweroff"
    },

    "WolframAlphaAppID": "optional-your-wolframalpha-app-id",

    "TwilioNumber": "+4912345678",
    "TwilioSID": "1234-5678",
    "TwilioAuthSecret": "abc-def",

	"TwitterConsumerKey": "1",
	"TwitterConsumerSecret": "2",
	"TwitterAccessToken": "3",
	"TwitterAccessSecret": "4",

    "MysteriousURL": "a",
    "MysteriousAddr1": "b",
    "MysteriousAddr2": "c",
    "MysteriousID1": "d",
    "MysteriousID2": "e"
}`
	var conf Config
	if err := json.Unmarshal([]byte(confText), &conf); err != nil {
		t.Fatal(err)
	}
	mailProc := conf.ToMailProcessor()
	if err := mailProc.CheckConfig(); err != nil {
		t.Fatal(err)
	}

	mailProcMatch := MailProcessor{
		CommandRunner: CommandRunner{
			SubHashSlashForPipe: true,
			SqueezeIntoOneLine:  true,
			TimeoutSec:          20,
			TruncateLen:         240,
			PIN:                 "MYSECRET",
			PresetMessages: map[string]string{
				"secretapple":     "echo hello world",
				"secretpineapple": "poweroff",
			},
			Mailer: Mailer{
				Recipients:     []string{"ITsupport@mydomain.com"},
				MailFrom:       "admin@mydomain.com",
				MTAAddressPort: "mydomain.com:25",
			},
			Twilio: TwilioClient{
				PhoneNumber: "+4912345678",
				AccountSID:  "1234-5678",
				AuthSecret:  "abc-def",
			},
			Twitter: TwitterClient{
				APIConsumerKey:       "1",
				APIConsumerSecret:    "2",
				APIAccessToken:       "3",
				APIAccessTokenSecret: "4",
			},
			WolframAlpha: WolframAlphaClient{AppID: "optional-your-wolframalpha-app-id"},
		},
		Mysterious: MysteriousClient{
			URL:   "a",
			Addr1: "b",
			Addr2: "c",
			ID1:   "d",
			ID2:   "e",
		},
	}
	if !reflect.DeepEqual(mailProc, mailProcMatch) {
		t.Fatalf("\n%+v\n%+v", mailProc, mailProcMatch)
	}

	webServer := conf.ToWebServer()
	if err := webServer.CheckConfig(); err != nil {
		t.Fatal(err)
	}
	webServerMatch := APIServer{
		MessageEndpoint:     "my_secret_endpoint_name_without_leading_slash",
		VoiceMLEndpoint:     "twilio_hook_initial_contact_without_leading_slash",
		VoiceProcEndpoint:   "twilio_hook_processor_without_leading_slash",
		VoiceEndpointPrefix: "/optional_voice_hook_proxy_prefix",
		ServerPort:          12321,
		TLSCert:             "/tmp/test.crt",
		TLSKey:              "/tmp/test.key",
		Command: CommandRunner{
			SubHashSlashForPipe: true,
			SqueezeIntoOneLine:  true,
			TimeoutSec:          10,
			TruncateLen:         120,
			PIN:                 "MYSECRET",
			PresetMessages: map[string]string{
				"secretapple":     "echo hello world",
				"secretpineapple": "poweroff",
			},
			Mailer: Mailer{
				Recipients:     []string{"ITsupport@mydomain.com"},
				MailFrom:       "admin@mydomain.com",
				MTAAddressPort: "mydomain.com:25",
			},
			Twilio: TwilioClient{
				PhoneNumber: "+4912345678",
				AccountSID:  "1234-5678",
				AuthSecret:  "abc-def",
			},
			Twitter: TwitterClient{
				APIConsumerKey:       "1",
				APIConsumerSecret:    "2",
				APIAccessToken:       "3",
				APIAccessTokenSecret: "4",
			},
			WolframAlpha: WolframAlphaClient{AppID: "optional-your-wolframalpha-app-id"},
		},
	}
	if !reflect.DeepEqual(webServer, webServerMatch) {
		t.Fatalf("\n%+v\n%+v", webServer, webServerMatch)
	}

}
