package main

import (
	"fmt"
	"github.com/HouzuoGuo/websh/bridge"
	"github.com/HouzuoGuo/websh/frontend/httpd"
	"github.com/HouzuoGuo/websh/httpclient"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
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
		"MTAHost": "127.0.0.1",
		"MTAPort": 25
	},
	"HTTPDaemon": {
		"ListenAddress": "127.0.0.1",
		"ListenPort": 23486,
		"ServeIndexDocument": "/tmp/test-websh-index.html",
		"ServeDirectories": {
			"/my/dir": "/tmp/test-websh-dir"
		}
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
		"SelfTestEndpoint": "/test",
		"TwilioSMSEndpoint": "/sms",
		"TwilioCallEndpoint": "/call",
		"TwilioCallEndpointConfig": {
			"CallGreeting": "hello there"
		},
		"MailMeEndpoint": "/mailme",
		"MailMeEndpointConfig": {
			"Recipients": ["howard@localhost"]
		}
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
	// Create a temporary file for index
	indexFile := "/tmp/test-websh-index.html"
	defer os.Remove(indexFile)
	if err := ioutil.WriteFile(indexFile, []byte("this is index"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a temporary directory of file
	htmlDir := "/tmp/test-websh-dir"
	if err := os.MkdirAll(htmlDir, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(htmlDir)
	if err := ioutil.WriteFile(htmlDir+"/a.html", []byte("a html"), 0644); err != nil {
		t.Fatal(err)
	}

	httpDaemon := config.GetHTTPD()

	if len(httpDaemon.SpecialHandlers) != 5 {
		// 1 x self test, 1 x sms, 2 x call, 1 x mailme
		t.Fatal(httpDaemon.SpecialHandlers)
	}
	// Find the randomly generated endpoint name for twilio call callback
	var twilioCallbackEndpoint string
	for endpoint := range httpDaemon.SpecialHandlers {
		switch endpoint {
		case "/sms":
		case "/call":
		case "/test":
		case "/mailme":
		default:
			twilioCallbackEndpoint = endpoint
		}
	}
	t.Log("Twilio callback endpoint is located at", twilioCallbackEndpoint)
	go func() {
		if err := httpDaemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	addr := "http://127.0.0.1:23486"
	time.Sleep(2 * time.Second)

	// Index handle
	for _, location := range httpd.IndexLocations {
		resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+location)
		if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != "this is index" {
			t.Fatal(err, string(resp.Body), resp)
		}
	}
	// Directory handle
	resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != `<pre>
<a href="a.html">a.html</a>
</pre>
` {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir/a.html")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != "a html" {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Non-existent paths
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir/doesnotexist.html")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/doesnotexist")
	if err != nil || resp.StatusCode != http.StatusNotFound || len(resp.Body) != 0 {
		t.Fatal(err, string(resp.Body), resp)
	}

	// Specialised handle - self_test
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/test")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatal(err, string(resp.Body), resp)
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
	// MailMe
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"msg": {"又给你发了一个邮件"}}.Encode()),
	}, addr+"/mailme")
	if err != nil || resp.StatusCode != http.StatusOK ||
		(!strings.Contains(string(resp.Body), "发不出去") && !strings.Contains(string(resp.Body), "发出去了")) {
		t.Fatal(err, string(resp.Body))
	}

	// ============ Test mail processor ============
	mailproc := config.GetMailProcessor()
	pinMismatch := `From howard@localhost Sun Feb 26 18:17:34 2017
Return-Path: <howard@localhost>
X-Original-To: howard@localhost
Delivered-To: howard@localhost
Received: by localhost (Postfix, from userid 1000)
        id 542EA2421BD; Sun, 26 Feb 2017 18:17:34 +0100 (CET)
Date: Sun, 26 Feb 2017 18:17:34 +0100
To: howard@localhost
Subject: hi howard
User-Agent: Heirloom mailx 12.5 7/5/10
MIME-Version: 1.0
Content-Type: text/plain; charset=us-ascii
Content-Transfer-Encoding: 7bit
Message-Id: <20170226171734.542EA2421BD@localhost.>
From: howard@localhost (Howard Guo)
Status: R

PIN mismatch`
	if err := mailproc.Process([]byte(pinMismatch)); err != bridge.ErrPINAndShortcutNotFound {
		t.Fatal(err)
	}
	// Real MTA is required for the shortcut email test
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err == nil {
		shortcutMatch := `From howard@localhost Sun Feb 26 18:17:34 2017
Return-Path: <howard@localhost>
X-Original-To: howard@localhost
Delivered-To: howard@localhost
Received: by localhost (Postfix, from userid 1000)
        id 542EA2421BD; Sun, 26 Feb 2017 18:17:34 +0100 (CET)
Date: Sun, 26 Feb 2017 18:17:34 +0100
To: howard@localhost
Subject: hi howard
User-Agent: Heirloom mailx 12.5 7/5/10
MIME-Version: 1.0
Content-Type: text/plain; charset=us-ascii
Content-Transfer-Encoding: 7bit
Message-Id: <20170226171734.542EA2421BD@localhost.>
From: howard@localhost (Howard Guo)
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
