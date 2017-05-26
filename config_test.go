package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/HouzuoGuo/laitos/bridge"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/dnsd"
	"github.com/HouzuoGuo/laitos/frontend/httpd"
	"github.com/HouzuoGuo/laitos/frontend/smtpd"
	"github.com/HouzuoGuo/laitos/httpclient"
	"io/ioutil"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// Most of the daemon test cases are copied from their own unit tests.
func TestConfig(t *testing.T) {
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
  "DNSDaemon": {
    "AllowQueryIPPrefixes": [
      "127.0"
    ],
    "PerIPLimit": 10,
    "TCPListenAddress": "127.0.0.1",
    "TCPListenPort": 61211,
    "TCPForwardTo": "8.8.8.8:53",
    "UDPListenAddress": "127.0.0.1",
    "UDPListenPort": 61211,
    "UDPForwardTo": "8.8.8.8:53"
  },
  "HTTPDaemon": {
    "ListenAddress": "127.0.0.1",
    "ListenPort": 23486,
    "BaseRateLimit": 10,
    "ServeDirectories": {
      "/my/dir": "/tmp/test-laitos-dir2"
    }
  },
  "HealthCheck": {
    "TCPPorts": [
      9114
    ],
    "IntervalSec": 300,
    "Recipients": [
      "howard@localhost"
    ]
  },
  "HTTPBridges": {
    "TranslateSequences": {
      "Sequences": [
        [
          "alpha",
          "beta"
        ]
      ]
    },
    "PINAndShortcuts": {
      "PIN": "httpsecret",
      "Shortcuts": {
        "httpshortcut": ".secho httpshortcut"
      }
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
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
    "InformationEndpoint": "/info",
    "CommandFormEndpoint": "/cmd_form",
    "GitlabBrowserEndpoint": "/gitlab",
    "GitlabBrowserEndpointConfig": {
      "PrivateToken": "just a dummy token"
    },
    "IndexEndpoints": [
      "/",
      "/index.html"
    ],
    "IndexEndpointConfig": {
      "HTMLFilePath": "/tmp/test-laitos-index2.html"
    },
    "MailMeEndpoint": "/mail_me",
    "MailMeEndpointConfig": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "WebProxyEndpoint": "/proxy",
    "TwilioSMSEndpoint": "/sms",
    "TwilioCallEndpoint": "/call",
    "TwilioCallEndpointConfig": {
      "CallGreeting": "Hi there"
    }
  },
  "MailDaemon": {
    "ListenAddress": "127.0.0.1",
    "ListenPort": 18573,
    "PerIPLimit": 10,
    "ForwardTo": [
      "howard@localhost",
      "root@localhost"
    ],
    "MyDomains": [
      "example.com",
      "howard.name"
    ]
  },
  "MailProcessor": {
    "CommandTimeoutSec": 10
  },
  "MailProcessorBridges": {
    "TranslateSequences": {
      "Sequences": [
        [
          "aaa",
          "bbb"
        ]
      ]
    },
    "PINAndShortcuts": {
      "PIN": "mailsecret",
      "Shortcuts": {
        "mailshortcut": ".secho mailshortcut"
      }
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "LintText": {
      "TrimSpaces": true,
      "CompressToSingleLine": true,
      "MaxLength": 70
    }
  },
  "SockDaemon": {
    "ListenAddress": "127.0.0.1",
    "ListenPort": 6891,
    "PerIPLimit": 10,
    "Password": "1234567"
  },
  "TelegramBot": {
    "AuthorizationToken": "intentionally-bad-token"
  },
  "TelegramBotBridges": {
    "TranslateSequences": {
      "Sequences": [
        [
          "123",
          "456"
        ]
      ]
    },
    "PINAndShortcuts": {
      "PIN": "telegramsecret",
      "Shortcuts": {
        "telegramshortcut": ".secho telegramshortcut"
      }
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "LintText": {
      "TrimSpaces": true,
      "CompressToSingleLine": true,
      "MaxLength": 120
    }
  }
}
`
	var config Config
	if err := config.DeserialiseFromJSON([]byte(js)); err != nil {
		t.Fatal(err)
	}

	DNSDaemonUDPTest(t, config)
	DNSDaemonTCPTest(t, config)
	HealthCheckTest(t, config)
	HTTPDaemonTest(t, config)
	InsecureHTTPDaemonTest(t, config)
	MailProcessorTest(t, config)
	SMTPDaemonTest(t, config)
	SockDaeemonTest(t, config)
	TelegramBotTest(t, config)
}

func DNSDaemonUDPTest(t *testing.T, config Config) {
	githubComUDPQuery, err := hex.DecodeString("e575012000010000000000010667697468756203636f6d00000100010000291000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	dnsDaemon := config.GetDNSD()
	// Update ad-server blacklist
	if entries, err := dnsDaemon.GetAdBlacklistPGL(); err != nil || len(entries) < 100 {
		t.Fatal(err, entries)
	}
	if entries, err := dnsDaemon.GetAdBlacklistMVPS(); err != nil || len(entries) < 100 {
		t.Fatal(err, entries)
	}
	// Server should start within two seconds
	go func() {
		if err := dnsDaemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)

	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:61211")
	if err != nil {
		t.Fatal(err)
	}
	packetBuf := make([]byte, dnsd.MaxPacketSize)
	// Try to reach rate limit
	var success int
	for i := 0; i < 40; i++ {
		go func() {
			clientConn, err := net.DialUDP("udp", nil, serverAddr)
			if err != nil {
				t.Fatal(err)
			}
			defer clientConn.Close()
			if err := clientConn.SetDeadline(time.Now().Add((dnsd.RateLimitIntervalSec - 1) * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := clientConn.Write(githubComUDPQuery); err != nil {
				t.Fatal(err)
			}
			length, err := clientConn.Read(packetBuf)
			fmt.Println("Read result", length, err)
			if err == nil && length > 50 {
				success++
			}
		}()
	}
	// Wait out rate limit
	time.Sleep(dnsd.RateLimitIntervalSec * time.Second)
	if success < 5 || success > 15 {
		t.Fatal(success)
	}
	// Blacklist github and see if query gets a black hole response
	dnsDaemon.BlackList["github.com"] = struct{}{}
	// This test is flaky and I do not understand why
	var blackListSuccess bool
	for i := 0; i < 10; i++ {
		clientConn, err := net.DialUDP("udp", nil, serverAddr)
		if err != nil {
			t.Fatal(err)
		}
		if err := clientConn.SetDeadline(time.Now().Add((dnsd.RateLimitIntervalSec - 1) * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := clientConn.Write(githubComUDPQuery); err != nil {
			t.Fatal(err)
		}
		respLen, err := clientConn.Read(packetBuf)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Index(packetBuf[:respLen], dnsd.BlackHoleAnswer) != -1 {
			blackListSuccess = true
			break
		}
	}
	if !blackListSuccess {
		t.Fatal("did not answer to blacklist domain")
	}
}

func DNSDaemonTCPTest(t *testing.T, config Config) {
	githubComTCPQuery, err := hex.DecodeString("00274cc7012000010000000000010667697468756203636f6d00000100010000291000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	dnsDaemon := config.GetDNSD()
	packetBuf := make([]byte, dnsd.MaxPacketSize)
	success := 0
	// Try to reach rate limit
	for i := 0; i < 40; i++ {
		go func() {
			clientConn, err := net.Dial("tcp", "127.0.0.1:61211")
			if err != nil {
				t.Fatal(err)
			}
			defer clientConn.Close()
			if err := clientConn.SetDeadline(time.Now().Add((dnsd.RateLimitIntervalSec - 1) * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := clientConn.Write(githubComTCPQuery); err != nil {
				t.Fatal(err)
			}
			resp, err := ioutil.ReadAll(clientConn)
			fmt.Println("Read result", len(resp), err)
			if err == nil && len(resp) > 50 {
				success++
			}
		}()
	}
	// Wait out rate limit
	time.Sleep(dnsd.RateLimitIntervalSec * time.Second)
	if success < 5 || success > 15 {
		t.Fatal(success)
	}
	// Blacklist github and see if query gets a black hole response
	dnsDaemon.BlackList["github.com"] = struct{}{}
	// This test is flaky and I do not understand why
	var blackListSuccess bool
	for i := 0; i < 10; i++ {
		clientConn, err := net.Dial("tcp", "127.0.0.1:61211")
		if err != nil {
			t.Fatal(err)
		}
		if err := clientConn.SetDeadline(time.Now().Add((dnsd.RateLimitIntervalSec - 1) * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := clientConn.Write(githubComTCPQuery); err != nil {
			t.Fatal(err)
		}
		respLen, err := clientConn.Read(packetBuf)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Index(packetBuf[:respLen], dnsd.BlackHoleAnswer) != -1 {
			blackListSuccess = true
			break
		}
	}
	if !blackListSuccess {
		t.Fatal("did not answer to blacklist domain")
	}
}

func HealthCheckTest(t *testing.T, config Config) {
	go func() {
		listener, err := net.Listen("tcp", "127.0.0.1:9114")
		if err != nil {
			t.Fatal(err)
		}
		for {
			if _, err := listener.Accept(); err != nil {
				t.Fatal(err)
			}
		}
	}()
	// Port is now listening
	time.Sleep(1 * time.Second)
	check := config.GetHealthCheck()
	if !check.Execute() {
		t.Fatal("some check failed")
	}
	// Break a feature
	check.FeaturesToCheck.LookupByTrigger[".s"] = &feature.Shell{}
	if check.Execute() {
		t.Fatal("did not fail")
	}
	check.FeaturesToCheck.LookupByTrigger[".s"] = &feature.Shell{InterpreterPath: "/bin/bash"}
	// Expect checks to begin within a second
	if err := check.Initialise(); err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := check.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(1 * time.Second)
}

func HTTPDaemonTest(t *testing.T, config Config) {
	// (Essentially combine all cases of api_test.go and httpd_test.go)
	// Create a temporary file for index
	indexFile := "/tmp/test-laitos-index2.html"
	defer os.Remove(indexFile)
	if err := ioutil.WriteFile(indexFile, []byte("this is index #LAITOS_CLIENTADDR #LAITOS_3339TIME"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a temporary directory of file
	htmlDir := "/tmp/test-laitos-dir2"
	if err := os.MkdirAll(htmlDir, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(htmlDir)
	if err := ioutil.WriteFile(htmlDir+"/a.html", []byte("a html"), 0644); err != nil {
		t.Fatal(err)
	}

	httpDaemon := config.GetHTTPD()

	if len(httpDaemon.SpecialHandlers) != 10 {
		// 1 x sms, 2 x call, 1 x gitlab, 1 x mail me, 1 x proxy, 2 x index, 1 x cmd form, 1 x info
		// Sorry, have to skip browser and browser image tests without a good excuse.
		t.Fatal(httpDaemon.SpecialHandlers)
	}
	// Find the randomly generated endpoint name for twilio call callback
	var twilioCallbackEndpoint string
	for endpoint := range httpDaemon.SpecialHandlers {
		switch endpoint {
		case "/sms":
		case "/call":
		case "/cmd_form":
		case "/mail_me":
		case "/proxy":
		case "/info":
		case "/gitlab":
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
	for _, location := range []string{"/", "/index.html"} {
		resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+location)
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
	// Test hitting rate limits
	time.Sleep(httpd.RateLimitIntervalSec * time.Second)
	success := 0
	for i := 0; i < 200; i++ {
		resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/")
		expected := "this is index 127.0.0.1 " + time.Now().Format(time.RFC3339)
		if err == nil && resp.StatusCode == http.StatusOK && string(resp.Body) == expected {
			success++
		}
	}
	if success < 50 || success > 150 {
		t.Fatal(success)
	}
	// Wait till rate limits reset
	time.Sleep(httpd.RateLimitIntervalSec * time.Second)
	// System information
	basicAuth := map[string][]string{"Authorization": {"Basic Og=="}}
	resp, err = httpclient.DoHTTP(httpclient.Request{Header: basicAuth}, addr+"/info")
	if err == nil && resp.StatusCode == http.StatusOK && strings.Index(string(resp.Body), "Stack traces:") == -1 {
		t.Fatal(err, string(resp.Body))
	}
	// If system information tells about a feature failure, the failure would only originate from mailer
	mailFailure := ".m: dial tcp 127.0.0.1:25: getsockopt: connection refused"
	if resp.StatusCode == http.StatusInternalServerError && strings.Index(string(resp.Body), mailFailure) == -1 {
		t.Fatal(err, string(resp.Body), resp)
	}

	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "Stack traces:") {
		t.Fatal(err, string(resp.Body))
	}

	// Gitlab handle
	resp, err = httpclient.DoHTTP(httpclient.Request{Header: basicAuth}, addr+"/gitlab")
	if err != nil || resp.StatusCode != http.StatusOK || strings.Index(string(resp.Body), "Enter path to browse") == -1 {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Command Form
	resp, err = httpclient.DoHTTP(httpclient.Request{Header: basicAuth}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{Method: http.MethodPost}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"cmd": {"httpsecret.sls /"}}.Encode()),
	}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "bin") {
		t.Fatal(err, string(resp.Body))
	}
	// MailMe
	resp, err = httpclient.DoHTTP(httpclient.Request{Header: basicAuth}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{Method: http.MethodPost, Header: basicAuth}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"msg": {"又给你发了一个邮件"}}.Encode()),
	}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK ||
		(!strings.Contains(string(resp.Body), "发不出去") && !strings.Contains(string(resp.Body), "发出去了")) {
		t.Fatal(err, string(resp.Body))
	}
	// Web proxy
	// Normally the proxy should inject javascript into the page, but the home page does not look like HTML so proxy won't do that.
	resp, err = httpclient.DoHTTP(httpclient.Request{Header: basicAuth}, addr+"/proxy?u=http%%3A%%2F%%2F127.0.0.1%%3A23486%%2F")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.HasPrefix(string(resp.Body), "this is index") {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - exchange SMS with bad PIN
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"Body": {"pin mismatch"}}.Encode()),
	}, addr+"/sms")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, resp)
	}
	// Twilio - exchange SMS, the extra spaces around prefix and PIN do not matter.
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"Body": {"httpsecret .s echo 0123456789012345678901234567890123456789"}}.Encode()),
	}, addr+"/sms")
	expected := `<?xml version="1.0" encoding="UTF-8"?>
<Response><Message><![CDATA[01234567890123456789012345678901234]]></Message></Response>
`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, resp)
	}
	// Twilio - check phone call greeting
	resp, err = httpclient.DoHTTP(httpclient.Request{Header: basicAuth}, addr+"/call")
	expected = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say><![CDATA[Hi there]]></Say>
    </Gather>
</Response>
`, twilioCallbackEndpoint)
	if err != nil || string(resp.Body) != expected {
		t.Fatalf("%+v\n%s\n%s", err, string(resp.Body), expected)
	}
	// Twilio - check phone call response to DTMF
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Header: basicAuth,
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
		Header: basicAuth,
		//                                             h t tp s   e c  r  e t .   s    tr  u e
		Body: strings.NewReader(url.Values{"Digits": {"4480870777733222777338014207777087778833"}}.Encode()),
	}, addr+twilioCallbackEndpoint)
	expected = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say><![CDATA[EMPTY OUTPUT, repeat again, EMPTY OUTPUT, repeat again, EMPTY OUTPUT, over.]]></Say>
    </Gather>
</Response>
`, twilioCallbackEndpoint)
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body))
	}
}

func InsecureHTTPDaemonTest(t *testing.T, config Config) {
	// Create a temporary file for index
	indexFile := "/tmp/test-laitos-index2.html"
	defer os.Remove(indexFile)
	if err := ioutil.WriteFile(indexFile, []byte("this is index #LAITOS_CLIENTADDR #LAITOS_3339TIME"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a temporary directory of file
	htmlDir := "/tmp/test-laitos-dir2"
	if err := os.MkdirAll(htmlDir, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(htmlDir)
	if err := ioutil.WriteFile(htmlDir+"/a.html", []byte("a html"), 0644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("PORT", "23487")
	httpDaemon := config.GetInsecureHTTPD()

	// This daemon is not much different from the ordinary HTTP daemon, so just copy over some of the test cases.
	if len(httpDaemon.SpecialHandlers) != 10 {
		// 1 x sms, 2 x call, 1 x gitlab, 1 x mail me, 1 x proxy, 2 x index, 1 x cmd form, 1 x info
		// Sorry, have to skip browser and browser image tests without a good excuse.
		t.Fatal(httpDaemon.SpecialHandlers)
	}
	go func() {
		if err := httpDaemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	addr := "http://127.0.0.1:23487"
	time.Sleep(2 * time.Second)

	// Index handle
	for _, location := range []string{"/", "/index.html"} {
		resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+location)
		expected := "this is index 127.0.0.1 " + time.Now().Format(time.RFC3339)
		if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
			t.Fatal(err, string(resp.Body), resp)
		}
	}
	// Non-existent paths
	resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir/doesnotexist.html")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/doesnotexist")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, string(resp.Body), resp)
	}
	// System information
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/info")
	if err == nil && resp.StatusCode == http.StatusOK && strings.Index(string(resp.Body), "Stack traces:") == -1 {
		t.Fatal(err, string(resp.Body))
	}
}

func MailProcessorTest(t *testing.T, config Config) {
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

func SMTPDaemonTest(t *testing.T, config Config) {
	mailDaemon := config.GetMailDaemon()
	var stoppedNormally bool
	go func() {
		if err := mailDaemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stoppedNormally = true
	}()
	time.Sleep(3 * time.Second) // this really should be env.HTTPPublicIPTimeout * time.Second
	// Try to exceed rate limit
	testMessage := "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body"
	success := 0
	for i := 0; i < 100; i++ {
		if err := smtp.SendMail("127.0.0.1:18573", nil, "ClientFrom@localhost", []string{"ClientTo@howard.name"}, []byte(testMessage)); err == nil {
			success++
		}
	}
	if success < 5 || success > 15 {
		t.Fatal("delivered", success)
	}
	time.Sleep(smtpd.RateLimitIntervalSec * time.Second)
	// Send an ordinary mail to the daemon
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body"
	if err := smtp.SendMail("127.0.0.1:18573", nil, "ClientFrom@localhost", []string{"ClientTo@example.com"}, []byte(testMessage)); err != nil {
		t.Fatal(err)
	}
	// Send a mail that does not belong to this server's domain
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body"
	if err := smtp.SendMail("127.0.0.1:18573", nil, "ClientFrom@localhost", []string{"ClientTo@not-my-domain"}, []byte(testMessage)); strings.Index(err.Error(), "Bad address") == -1 {
		t.Fatal(err)
	}
	// Try run a command via email
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: command subject\r\n\r\nverysecret.s echo hi"
	if err := smtp.SendMail("127.0.0.1:18573", nil, "ClientFrom@localhost", []string{"ClientTo@howard.name"}, []byte(testMessage)); err != nil {
		t.Fatal(err)
	}
	t.Log("Check howard@localhost and root@localhost mailbox")
	// Daemon must stop in a second
	mailDaemon.Stop()
	time.Sleep(1 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}
}

func SockDaeemonTest(t *testing.T, config Config) {
	sockDaemon := config.GetSockDaemon()
	var stopped bool
	go func() {
		if err := sockDaemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stopped = true
	}()
	time.Sleep(2 * time.Second)
	if conn, err := net.Dial("tcp", "127.0.0.1:6891"); err != nil {
		t.Fatal(err)
	} else if n, err := conn.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}); err != nil && n != 10 {
		t.Fatal(err, n)
	}
	sockDaemon.Stop()
	time.Sleep(1 * time.Second)
	if !stopped {
		t.Fatal("did not stop")
	}
}

func TelegramBotTest(t *testing.T, config Config) {
	bot := config.GetTelegramBot()
	if err := bot.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := bot.StartAndBlock(); err == nil || strings.Index(err.Error(), "HTTP") == -1 {
		t.Fatal(err)
	}
}
