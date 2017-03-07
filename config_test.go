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
		"BaseRateLimit":10,
		"ServeDirectories": {
			"/my/dir": "/tmp/test-websh-dir2"
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
		},
		"WebProxyEndpoint": "/proxy",

		"IndexEndpoints": ["/", "/index.html"],
		"IndexEndpointConfig": {
			"HTMLFilePath": "/tmp/test-websh-index2.html"
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
	indexFile := "/tmp/test-websh-index2.html"
	defer os.Remove(indexFile)
	if err := ioutil.WriteFile(indexFile, []byte("this is index #WEBSH_CLIENTADDR #WEBSH_3339TIME"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a temporary directory of file
	htmlDir := "/tmp/test-websh-dir2"
	if err := os.MkdirAll(htmlDir, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(htmlDir)
	if err := ioutil.WriteFile(htmlDir+"/a.html", []byte("a html"), 0644); err != nil {
		t.Fatal(err)
	}

	httpDaemon := config.GetHTTPD()

	if len(httpDaemon.SpecialHandlers) != 8 {
		// 1 x self test, 1 x sms, 2 x call, 1 x mailme, 1 x proxy, 2 x index
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
		case "/proxy":
		case "/":
		case "/index.html":
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
	var indexRespBody string
	for _, location := range []string{"/", "/index.html"} {
		resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+location)
		indexRespBody = string(resp.Body)
		expected := "this is index 127.0.0.1 " + time.Now().Format(time.RFC3339)
		if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
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
	if err != nil || resp.StatusCode != http.StatusNotFound {
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
		t.Fatalf("%+v\n%s\n%s", err, string(resp.Body), expected)
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
	// Web proxy
	// Normally the proxy should inject javascript into the page, but the home page does not look like HTML so proxy won't do that.
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/proxy?u=http%%3A%%2F%%2F127.0.0.1%%3A13589%%2F")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != indexRespBody {
		t.Fatalf("Err: %v\nResp: %+v\nActual:%s\nExpected:%s\n", err, resp, string(resp.Body), indexRespBody)
	}
	// Test hitting rate limits
	time.Sleep(httpd.HTTPD_RATE_LIMIT_INTERVAL_SEC * time.Second)
	success := 0
	for i := 0; i < 200; i++ {
		resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/")
		expected := "this is index 127.0.0.1 " + time.Now().Format(time.RFC3339)
		if err == nil && resp.StatusCode == http.StatusOK && string(resp.Body) == expected {
			success++
		}
	}
	if success > 105 || success < 95 {
		t.Fatal(success)
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
