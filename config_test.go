package main

import (
	"github.com/HouzuoGuo/laitos/frontend/dnsd"
	"github.com/HouzuoGuo/laitos/frontend/healthcheck"
	"github.com/HouzuoGuo/laitos/frontend/httpd"
	"github.com/HouzuoGuo/laitos/frontend/mailp"
	"github.com/HouzuoGuo/laitos/frontend/plain"
	"github.com/HouzuoGuo/laitos/frontend/smtpd"
	"github.com/HouzuoGuo/laitos/frontend/sockd"
	"github.com/HouzuoGuo/laitos/frontend/telegrambot"
	"testing"
	"time"
)

// Most of the daemon test cases are copied from their own unit tests.
func TestConfig(t *testing.T) {
	js := `
{
  "DNSDaemon": {
    "Address": "127.0.0.1",
    "AllowQueryIPPrefixes": [
      "127.0"
    ],
    "PerIPLimit": 10,
    "TCPForwarder": "8.8.8.8:53",
    "TCPPort": 45115,
    "UDPForwarder": "8.8.8.8:53",
    "UDPPort": 23518
  },
  "Features": {
    "Shell": {
      "InterpreterPath": "/bin/bash"
    }
  },
  "HTTPBridges": {
    "LintText": {
      "CompressSpaces": true,
      "CompressToSingleLine": true,
      "KeepVisible7BitCharOnly": true,
      "MaxLength": 35,
      "TrimSpaces": true
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "PINAndShortcuts": {
      "PIN": "verysecret",
      "Shortcuts": {
        "httpshortcut": ".secho httpshortcut"
      }
    },
    "TranslateSequences": {
      "Sequences": [
        [
          "alpha",
          "beta"
        ]
      ]
    }
  },
  "HTTPDaemon": {
    "Address": "127.0.0.1",
    "BaseRateLimit": 10,
    "Port": 23486,
    "ServeDirectories": {
      "/my/dir": "/tmp/test-laitos-dir"
    }
  },
  "HTTPHandlers": {
    "CommandFormEndpoint": "/cmd_form",
    "GitlabBrowserEndpoint": "/gitlab",
    "GitlabBrowserEndpointConfig": {
      "PrivateToken": "just a dummy token"
    },
    "IndexEndpointConfig": {
      "HTMLFilePath": "/tmp/test-laitos-index.html"
    },
    "IndexEndpoints": [
      "/html",
      "/"
    ],
    "InformationEndpoint": "/info",
    "MailMeEndpoint": "/mail_me",
    "MailMeEndpointConfig": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "TwilioCallEndpoint": "/call_greeting",
    "TwilioCallEndpointConfig": {
      "CallGreeting": "Hi there"
    },
    "TwilioSMSEndpoint": "/sms",
    "WebProxyEndpoint": "/proxy"
  },
  "HealthCheck": {
    "IntervalSec": 300,
    "Recipients": [
      "howard@localhost"
    ],
    "TCPPorts": [
      9114
    ]
  },
  "MailBridges": {
    "LintText": {
      "CompressToSingleLine": true,
      "MaxLength": 70,
      "TrimSpaces": true
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "PINAndShortcuts": {
      "PIN": "verysecret",
      "Shortcuts": {
        "mailshortcut": ".secho mailshortcut"
      }
    },
    "TranslateSequences": {
      "Sequences": [
        [
          "aaa",
          "bbb"
        ]
      ]
    }
  },
  "MailDaemon": {
    "Address": "127.0.0.1",
    "ForwardTo": [
      "howard@localhost",
      "root@localhost"
    ],
    "MyDomains": [
      "example.com",
      "howard.name"
    ],
    "PerIPLimit": 10,
    "Port": 18573
  },
  "MailProcessor": {
    "CommandTimeoutSec": 10
  },
  "Mailer": {
    "MTAHost": "127.0.0.1",
    "MTAPort": 25,
    "MailFrom": "howard@localhost"
  },
  "PlainTextBridges": {
    "LintText": {
      "CompressToSingleLine": false,
      "MaxLength": 120,
      "TrimSpaces": true
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "PINAndShortcuts": {
      "PIN": "verysecret",
      "Shortcuts": {
        "telegramshortcut": ".secho plaintextshortcut"
      }
    },
    "TranslateSequences": {
      "Sequences": [
        [
          "iii",
          "jjj"
        ]
      ]
    }
  },
  "PlainTextDaemon": {
    "Address": "127.0.0.1",
    "PerIPLimit": 10,
    "TCPPort": 17011,
    "UDPPort": 43915
  },
  "SockDaemon": {
    "Address": "127.0.0.1",
    "Password": "1234567",
    "PerIPLimit": 10,
    "TCPPort": 6891,
    "UDPPort": 9122
  },
  "TelegramBot": {
    "AuthorizationToken": "intentionally-bad-token",
    "RateLimit": 10
  },
  "TelegramBridges": {
    "LintText": {
      "CompressToSingleLine": true,
      "MaxLength": 120,
      "TrimSpaces": true
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "PINAndShortcuts": {
      "PIN": "verysecret",
      "Shortcuts": {
        "telegramshortcut": ".secho telegramshortcut"
      }
    },
    "TranslateSequences": {
      "Sequences": [
        [
          "123",
          "456"
        ]
      ]
    }
  }
}
`
	var config Config
	if err := config.DeserialiseFromJSON([]byte(js)); err != nil {
		t.Fatal(err)
	}

	dnsDaemon := config.GetDNSD()
	dnsd.TestUDPQueries(dnsDaemon, t)
	dnsd.TestTCPQueries(dnsDaemon, t)

	healthcheck.TestHealthCheck(config.GetHealthCheck(), t)

	httpDaemon := config.GetHTTPD()
	// HTTP daemon is expected to start in two seconds
	go func() {
		if err := httpDaemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)
	httpd.TestHTTPD(httpDaemon, t)
	httpd.TestAPIHandlers(httpDaemon, t)

	insecureHTTPDaemon := config.GetInsecureHTTPD()
	// Insecure HTTP daemon should listen on port 80 in deployment
	if insecureHTTPDaemon.Port != 80 {
		t.Fatal("wrong port for insecure HTTP daemon to listen on")
	}
	// However, this test case does not run as root, so give it an unprivileged port.
	insecureHTTPDaemon.Port = 51991
	// Re-initialise internal states to make new port number effective
	if err := insecureHTTPDaemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Insecure HTTP daemon is expected to start in two seconds
	go func() {
		if err := insecureHTTPDaemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)
	httpd.TestHTTPD(insecureHTTPDaemon, t)
	httpd.TestAPIHandlers(insecureHTTPDaemon, t)

	mailp.TestMailp(config.GetMailProcessor(), t)

	smtpd.TestSMTPD(config.GetMailDaemon(), t)

	plain.TestTCPServer(config.GetPlainTextDaemon(), t)
	plain.TestUDPServer(config.GetPlainTextDaemon(), t)

	sockd.TestSockd(config.GetSockDaemon(), t)

	telegrambot.TestTelegramBot(config.GetTelegramBot(), t)
}
