package main

import (
	"github.com/HouzuoGuo/laitos/frontend/dnsd"
	"github.com/HouzuoGuo/laitos/frontend/healthcheck"
	"github.com/HouzuoGuo/laitos/frontend/httpd"
	"github.com/HouzuoGuo/laitos/frontend/mailp"
	"github.com/HouzuoGuo/laitos/frontend/smtpd"
	"github.com/HouzuoGuo/laitos/frontend/sockd"
	"github.com/HouzuoGuo/laitos/frontend/telegrambot"
	"testing"
)

// Most of the daemon test cases are copied from their own unit tests.
func TestConfig(t *testing.T) {
	js := `
{
  "DNSDaemon": {
    "AllowQueryIPPrefixes": [
      "127.0"
    ],
    "PerIPLimit": 10,
    "TCPForwardTo": "8.8.8.8:53",
    "TCPListenAddress": "127.0.0.1",
    "TCPListenPort": 45115,
    "UDPForwardTo": "8.8.8.8:53",
    "UDPListenAddress": "127.0.0.1",
    "UDPListenPort": 23518
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
    "BaseRateLimit": 10,
    "ListenAddress": "127.0.0.1",
    "ListenPort": 23486,
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
  "MailDaemon": {
    "ForwardTo": [
      "howard@localhost",
      "root@localhost"
    ],
    "ListenAddress": "127.0.0.1",
    "ListenPort": 18573,
    "MyDomains": [
      "example.com",
      "howard.name"
    ],
    "PerIPLimit": 10
  },
  "MailProcessor": {
    "CommandTimeoutSec": 10
  },
  "MailProcessorBridges": {
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
  "Mailer": {
    "MTAHost": "127.0.0.1",
    "MTAPort": 25,
    "MailFrom": "howard@localhost"
  },
  "SockDaemon": {
    "ListenAddress": "127.0.0.1",
    "ListenPort": 6891,
    "Password": "1234567",
    "PerIPLimit": 10
  },
  "TelegramBot": {
    "AuthorizationToken": "intentionally-bad-token"
  },
  "TelegramBotBridges": {
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
	httpd.TestHTTPD(httpDaemon, t)
	httpd.TestAPIHandlers(httpDaemon, t)

	insecureHTTPDaemon := config.GetInsecureHTTPD()
	// Insecure HTTP daemon should listen on port 80 in deployment
	if insecureHTTPDaemon.ListenPort != 80 {
		t.Fatal("wrong port for insecure HTTP daemon to listen on")
	}
	// However, this test case does not run as root, so give it an unprivileged port.
	insecureHTTPDaemon.ListenPort = 51991
	// Re-initialise internal states to make new port number effective
	if err := insecureHTTPDaemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	httpd.TestHTTPD(insecureHTTPDaemon, t)
	httpd.TestAPIHandlers(insecureHTTPDaemon, t)

	mailp.TestMailp(config.GetMailProcessor(), t)

	smtpd.TestSMTPD(config.GetMailDaemon(), t)

	sockd.TestSockd(config.GetSockDaemon(), t)

	telegrambot.TestTelegramBot(config.GetTelegramBot(), t)
}
