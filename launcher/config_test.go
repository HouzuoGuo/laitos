package launcher

import (
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/autounlock"
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/daemon/httpd"
	"github.com/HouzuoGuo/laitos/daemon/httpproxy"
	"github.com/HouzuoGuo/laitos/daemon/maintenance"
	"github.com/HouzuoGuo/laitos/daemon/passwdrpc"
	"github.com/HouzuoGuo/laitos/daemon/plainsocket"
	"github.com/HouzuoGuo/laitos/daemon/simpleipsvcd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/daemon/snmpd"
	"github.com/HouzuoGuo/laitos/daemon/sockd"
	"github.com/HouzuoGuo/laitos/daemon/telegrambot"
	"github.com/HouzuoGuo/laitos/misc"
)

var sampleConfigJSON = `
{
  "Features": {
    "Shell": {
      "Unrestricted": true
    }
  },
  "AutoUnlock": {
    "IntervalSec": 30,
    "URLAndPassword": {
      "http://example.com/does-not-matter": "password does not matter"
    }
  },
  "DNSDaemon": {
    "Address": "127.0.0.1",
    "AllowQueryFromCidrs": [
      "192.0.0.0/8"
    ],
    "MyDomainNames": ["example.com", "example.net"],
    "CustomRecords": {
      "example.net": {
        "TXT": {
          "Entries": [
            "v=spf1 mx a mx:hz.gl mx:howard.gg mx:houzuo.net ?all",
            "apple-domain-verification=Abcdefg1234567"
          ]
        },
        "MX": [
          {"Pref": 10, "Host": "mx1.example.net"},
          {"Pref": 20, "Host": "mx2.example.net"}
        ]
      },
      "ns-other.example.net": {
        "NS": {
          "Names": ["ns.other.example.com"]
        },
        "A": {
          "CanonicalName": "example.com"
        }
      },
      "example.com": {
        "A": {
          "Addresses": ["5.0.0.1", "5.0.0.2"]
        },
        "AAAA": {
          "Addresses": ["::5", "::6"]
        }
      }
    },
    "PerIPLimit": 40,
    "TCPPort": 45115,
    "UDPPort": 23518
  },
  "DNSFilters": {
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
      "Passwords": ["verysecret"],
      "Shortcuts": {
        "dnsshortcut": ".secho dnsshortcut"
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
  },
  "HTTPDaemon": {
    "Address": "127.0.0.1",
    "PerIPLimit": 10,
    "Port": 23486,
    "ServeDirectories": {
      "/dir": "/tmp/test-laitos-dir",
      "/my/dir": "/tmp/test-laitos-dir"
    }
  },
  "HTTPProxyDaemon": {
    "Address": "127.0.0.1",
    "PerIPLimit": 10,
    "Port": 54112,
    "AllowFromCidrs": ["127.0.0.0/8"]
  },
  "HTTPFilters": {
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
      "Passwords": ["verysecret"],
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
  "HTTPHandlers": {
    "CommandFormEndpoint": "/cmd_form",
    "FileUploadEndpoint": "/upload",
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
    "RecurringCommandsEndpoint": "/recurring_cmds",
    "RecurringCommandsEndpointConfig": {
      "RecurringCommands": {
        "channel1": {
          "IntervalSec": 1,
          "MaxResults": 4,
          "PreConfiguredCommands": [
            "verysecret.s echo -n this is channel1"
          ]
        },
        "channel2": {
          "IntervalSec": 1,
          "MaxResults": 4,
          "PreConfiguredCommands": [
            "verysecret.s echo -n this is channel2"
          ]
        }
      }
    },
    "LoraWANWebhookEndpoint": "/ttn",
    "TwilioCallEndpoint": "/call_greeting",
    "TwilioCallEndpointConfig": {
      "CallGreeting": "Hi there"
    },
    "TwilioSMSEndpoint": "/sms",
    "WebProxyEndpoint": "/proxy",
    "AppCommandEndpoint": "/cmd",
    "ReportsRetrievalEndpoint": "/reports",
    "ProcessExplorerEndpoint": "/procexp"
  },
  "MailClient": {
    "MTAHost": "127.0.0.1",
    "MTAPort": 25,
    "MailFrom": "howard@localhost"
  },
  "MailCommandRunner": {
    "CommandTimeoutSec": 10
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
    "PerIPLimit": 5,
    "Port": 18573
  },
  "MailFilters": {
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
      "Passwords": ["verysecret"],
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
  "Maintenance": {
    "BlockSystemLoginExcept": [
      "Administrator",
      "root",
      "howard"
    ],
    "DisableStopServices": [
      "does-not-exist"
    ],
    "DoEnhanceFileSecurity": true,
    "EnableStartServices": [
      "does-not-exist"
    ],
    "InstallPackages": [
      "htop"
    ],
    "IntervalSec": 86400,
    "ScriptForUnix": "touch /tmp/laitos-maintenance-pre-script-test",
    "Recipients": [
      "howard@localhost"
    ],
    "SetTimeZone": "UTC",
    "SwapFileSizeMB": 100,
    "TCPPorts": [
      9114
    ],
    "TuneLinux": true
  },
  "PasswordRPCDaemon": {
    "Address": "0.0.0.0",
    "Port": 18378
  },
  "PhoneHomeDaemon": {
    "MessageProcessorServers": [
      {
        "HTTPEndpointURL": "dummy",
        "Passwords": ["dummy"]
      }
    ],
    "ReportIntervalSec": 1
  },
  "PhoneHomeFilters": {
    "LintText": {
      "CompressToSingleLine": false,
      "MaxLength": 1000,
      "TrimSpaces": true
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "PINAndShortcuts": {
      "Passwords": ["verysecret"],
      "Shortcuts": {
        "plainsocketshortcut": ".secho plainsockethortcut"
      }
    }
  },
  "PlainSocketDaemon": {
    "Address": "127.0.0.1",
    "PerIPLimit": 5,
    "TCPPort": 17011,
    "UDPPort": 43915
  },
  "PlainSocketFilters": {
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
      "Passwords": ["verysecret"],
      "Shortcuts": {
        "plainsocketshortcut": ".secho plainsockethortcut"
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
  "SNMPDaemon": {
    "CommunityName": "public",
    "Port": 33210
  },
  "SimpleIPSvcDaemon": {
    "ActiveUserNames": "howard (houzuo) guo",
    "ActiveUsersPort": 16222,
    "DayTimePort": 62989,
    "QOTD": "hello from howard",
    "QOTDPort": 59594
  },
  "SockDaemon": {
    "Address": "127.0.0.1",
    "Password": "1234567",
    "PerIPLimit": 10,
    "TCPPorts": [
      6891,
      8837
    ],
    "UDPPorts": [
      9122,
      24899
    ]
  },
  "SupervisorNotificationRecipients": [
    "howard@localhost"
  ],
  "TelegramBot": {
    "AuthorizationToken": "intentionally-bad-token",
    "PerUserLimit": 2
  },
  "TelegramFilters": {
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
      "Passwords": ["verysecret"],
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

func TestConfig(t *testing.T) {
	misc.EnableAWSIntegration = true
	misc.EnablePrometheusIntegration = true
	var config Config
	if err := config.DeserialiseFromJSON([]byte(sampleConfigJSON)); err != nil {
		t.Fatal(err)
	}

	httpd.PrepareForTestHTTPD(t)
	httpDaemon := config.GetHTTPD()
	go func() {
		if err := httpDaemon.StartAndBlockNoTLS(0); err != nil {
			t.Error(err)
			return
		}
	}()
	if !misc.ProbePort(30*time.Second, httpDaemon.Address, httpDaemon.Port) {
		t.Fatal("server did not start in time")
	}

	httpd.TestHTTPD(httpDaemon, t)
	httpd.TestAPIHandlers(httpDaemon, t)

	dnsDaemon := config.GetDNSD()
	dnsd.TestServer(dnsDaemon, t)

	maintenance.TestMaintenance(config.GetMaintenance(), t)

	if err := config.GetMailCommandRunner().Initialise(); err != nil {
		t.Fatal(err)
	}
	mailcmd.TestCommandRunner(config.GetMailCommandRunner(), t)

	smtpd.TestSMTPD(config.GetMailDaemon(), t)

	plainsocket.TestServer(config.GetPlainSocketDaemon(), t)

	sockd.TestSockd(config.GetSockDaemon(), t)

	simpleipsvcd.TestSimpleIPSvcD(config.GetSimpleIPSvcD(), t)

	snmpd.TestSNMPD(config.GetSNMPD(), t)

	telegrambot.TestTelegramBot(config.GetTelegramBot(), t)

	autounlock.TestAutoUnlock(config.GetAutoUnlock(), t)

	passwdrpc.TestPasswdRPCDaemon(config.GetPasswdRPCDaemon(), t)

	httpproxy.TestHTTPProxyDaemon(config.GetHTTPProxyDaemon(), t)
}
