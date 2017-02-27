package main

import (
	"fmt"
	"github.com/HouzuoGuo/websh/bridge"
	"github.com/HouzuoGuo/websh/httpclient"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestConfig(t *testing.T) {
	// Configure HTTP daemon and mail processor for running shell commands
	js := `{
	"Features": {
		"Shell": {
			"InterpreterPath": "/bin/bash"
		}
	},
	"Mailer": {
		"MailFrom": "howard@localhost",
		"MTAAddressPort": "127.0.0.1:25"
	},
	"HTTPDaemon": {
		"ListenAddress": "127.0.0.1",
		"ListenPort": 23486
	},
	"HTTPBridges": {
		"TranslateSequences": {
			"Sequences": [
				["alpha", "beta"]
			]
		},
		"PINAndShortcuts": {
			"PIN": "httpsecret",
			"Shortcuts": {
				"httpshortcut": ".secho alpha"
			}
		},
		"NotifyViaEmail": {
			"Recipients": ["howard@localhost"]
		},
		"LintText": {
			"TrimSpaces": true,
			"CompressToSingleLine": true,
			"KeepVisible7BitCharOnly": true,
			"CompressSpaces": true,
			"MaxLength": 35
		}
	},
	"HTTPHandlers": {
		"TwilioSMSEndpoint": "/sms",
		"TwilioCallEndpoint": "/call",
		"TwilioCallEndpointConfig": {
			"CallGreeting": "hello there"
		},
		"SelfTestEndpoint": "/test"
	},
	"MailProcessor": {
		"CommandTimeoutSec": 10
	},
	"MailProcessorBridges": {
		"TranslateSequences": {
		"Sequences": [
				["aaa", "bbb"]
			]
		},
		"PINAndShortcuts": {
			"PIN": "mailsecret",
			"Shortcuts": {
				"mailshortcut": ".secho aaa"
			}
		},
		"NotifyViaEmail": {
			"Recipients": ["howard@localhost"]
		},
		"LintText": {
			"TrimSpaces": true,
			"CompressToSingleLine": true,
			"MaxLength": 35
		}
	}
}`
	var config Config
	if err := config.DeserialiseFromJSON([]byte(js)); err != nil {
		t.Fatal(err)
	}

	// ============ Test HTTP daemon ============

	httpDaemon := config.GetHTTPD()

	if len(httpDaemon.Handlers) != 4 {
		// 2 x call, 1 x sms, 1 x self test
		t.Fatal(httpDaemon.Handlers)
	}
	// Find the randomly generated endpoint name for twilio call callback
	var twilioCallbackEndpoint string
	for endpoint := range httpDaemon.Handlers {
		switch endpoint {
		case "/sms":
		case "/call":
		case "/test":
		default:
			twilioCallbackEndpoint = endpoint
		}
	}
	t.Log("Twilio callback endpoint is located at ", twilioCallbackEndpoint)
	go func() {
		if err := httpDaemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)

	// Run feature test
	addr := "http://127.0.0.1:23486"
	resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+"/test")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatal(err, resp)
	}

	// Twilio - exchange SMS
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"Body": {"badsecret.secho alpha"}}.Encode()),
	}, addr+"/sms")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"Body": {"httpsecret.secho alpha"}}.Encode()),
	}, addr+"/sms")
	expected := `<?xml version="1.0" encoding="UTF-8"?>
<Response><Message>beta</Message></Response>
`
	if err != nil || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - check phone call greeting
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/call")
	expected = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say>hello there</Say>
    </Gather>
</Response>
`, twilioCallbackEndpoint)
	if err != nil || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - check phone call response to DTMF
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"Digits": {"0000000"}}.Encode()),
	}, addr+twilioCallbackEndpoint)
	expected = `<?xml version="1.0" encoding="UTF-8"?>
<Response>
	<Say>Sorry</Say>
	<Hangup/>
</Response>
`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - check phone call response to command
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		//                                             h t tp s   e c  r  e t .   s    tr  u e
		Body: strings.NewReader(url.Values{"Digits": {"4480870777733222777338014207777087778833"}}.Encode()),
	}, addr+twilioCallbackEndpoint)
	expected = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say>EMPTY OUTPUT, repeat again, EMPTY OUTPUT, repeat again, EMPTY OUTPUT, over.</Say>
    </Gather>
</Response>
`, twilioCallbackEndpoint)
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body))
	}

	// ============ Test mail processor ============
	mailproc := config.GetMailProcessor()
	pinMismatch := `From howard@linux-mtj3 Sun Feb 26 18:17:34 2017
Return-Path: <howard@linux-mtj3>
X-Original-To: howard@localhost
Delivered-To: howard@localhost
Received: by linux-mtj3 (Postfix, from userid 1000)
        id 542EA2421BD; Sun, 26 Feb 2017 18:17:34 +0100 (CET)
Date: Sun, 26 Feb 2017 18:17:34 +0100
To: howard@localhost
Subject: hi howard
User-Agent: Heirloom mailx 12.5 7/5/10
MIME-Version: 1.0
Content-Type: text/plain; charset=us-ascii
Content-Transfer-Encoding: 7bit
Message-Id: <20170226171734.542EA2421BD@linux-mtj3.>
From: howard@linux-mtj3 (Howard Guo)
Status: R

PIN mismatch`
	if err := mailproc.Process([]byte(pinMismatch)); err != bridge.ErrPINAndShortcutNotFound {
		t.Fatal(err)
	}
	// Real MTA is required for the shortcut email test
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err == nil {
		shortcutMatch := `From howard@linux-mtj3 Sun Feb 26 18:17:34 2017
Return-Path: <howard@linux-mtj3>
X-Original-To: howard@localhost
Delivered-To: howard@localhost
Received: by linux-mtj3 (Postfix, from userid 1000)
        id 542EA2421BD; Sun, 26 Feb 2017 18:17:34 +0100 (CET)
Date: Sun, 26 Feb 2017 18:17:34 +0100
To: howard@localhost
Subject: hi howard
User-Agent: Heirloom mailx 12.5 7/5/10
MIME-Version: 1.0
Content-Type: text/plain; charset=us-ascii
Content-Transfer-Encoding: 7bit
Message-Id: <20170226171734.542EA2421BD@linux-mtj3.>
From: howard@linux-mtj3 (Howard Guo)
Status: R

PIN mismatch
mailshortcut
`
		if err := mailproc.Process([]byte(shortcutMatch)); err != nil {
			t.Fatal(err)
		}
		t.Log("Check howard@localhost mailbox")
	}
}
