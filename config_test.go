package main

import (
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/daemon/httpd"
	"github.com/HouzuoGuo/laitos/daemon/maintenance"
	"github.com/HouzuoGuo/laitos/daemon/plainsockets"
	"github.com/HouzuoGuo/laitos/daemon/smtpd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/daemon/sockd"
	"github.com/HouzuoGuo/laitos/daemon/telegrambot"
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
      "192"
    ],
    "PerIPLimit": 10,
    "TCPForwarders": ["8.8.8.8:53", "8.8.4.4:53", "185.121.177.177:53", "169.239.202.202:53", "8.26.56.26:53", "8.20.247.20:53"],
    "TCPPort": 45115,
    "UDPForwarders": ["8.8.8.8:53", "8.8.4.4:53", "185.121.177.177:53", "169.239.202.202:53", "8.26.56.26:53", "8.20.247.20:53"],
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
      "/my/dir": "/tmp/test-laitos-dir",
      "/dir": "/tmp/test-laitos-dir"
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
	"MicrosoftBotEndpoint1": "/microsoft_bot",
    "MicrosoftBotEndpointConfig1": {
        "ClientAppID": "dummy id",
        "ClientAppSecret": "dummy secret"
    },
    "TwilioCallEndpoint": "/call_greeting",
    "TwilioCallEndpointConfig": {
      "CallGreeting": "Hi there"
    },
    "TwilioSMSEndpoint": "/sms",
    "WebProxyEndpoint": "/proxy"
  },
  "Maintenance": {
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
  "MailClient": {
    "MTAHost": "127.0.0.1",
    "MTAPort": 25,
    "MailFrom": "howard@localhost"
  },
  "PlainSocketBridges": {
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
        "telegramshortcut": ".secho plainsocketshortcut"
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
  "PlainSocketDaemon": {
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

	maintenance.TestMaintenance(config.GetMaintenance(), t)

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

	mailcmd.TestCommandRunner(config.GetMailCommandRunner(), t)

	smtpd.TestSMTPD(config.GetMailDaemon(), t)

	plainsockets.TestTCPServer(config.GetPlainSocketDaemon(), t)
	plainsockets.TestUDPServer(config.GetPlainSocketDaemon(), t)

	sockd.TestSockd(config.GetSockDaemon(), t)

	telegrambot.TestTelegramBot(config.GetTelegramBot(), t)
}
