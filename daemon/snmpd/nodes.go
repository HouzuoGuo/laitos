package snmpd

import (
	"encoding/asn1"
	"github.com/HouzuoGuo/laitos/daemon/autounlock"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/httpd/handler"
	"github.com/HouzuoGuo/laitos/daemon/smtpd"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"runtime"
	"time"
)

/*
	OIDLaitos is the very top of OID hierarchy used in laitos SNMP server.
	It contains a private enterprise number registered by Houzuo (Howard) Guo, 52535:
	{iso(1) identified-organization(3) dod(6) internet(1) private(4) enterprise(1) 52535}

	And laitos occupies number 121 underneath it.
*/
var OIDLaitos = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 52535, 121}

// OIDNode is a function that retrieves a latest system/application performance indicator value.
type OIDNode func() interface{}

var (
	// OIDLaitosNodes is a comprehensive list of suffix OIDs that correspond to nodes supported by laitos SNMP server.
	OIDLaitosNodes = map[int]OIDNode{
		// 1.3.6.1.4.1.52535.121.100 Octet string - public IP address
		100: func() interface{} {
			return []byte(inet.GetPublicIP())
		},
		// 1.3.6.1.4.1.52535.121.101 Date time - system clock in UTC
		101: func() interface{} {
			return time.Now().In(time.UTC)
		},
		// 1.3.6.1.4.1.52535.121.102 Integer - number of seconds program has been running
		102: func() interface{} {
			return int64(time.Now().Sub(misc.StartupTime).Seconds())
		},
		// 1.3.6.1.4.1.52535.121.103 Integer - number of CPUs
		103: func() interface{} {
			return int64(runtime.NumCPU())
		},
		// 1.3.6.1.4.1.52535.121.104 Integer - GOMAXPROCS
		104: func() interface{} {
			return int64(runtime.GOMAXPROCS(-1))
		},
		// 1.3.6.1.4.1.52535.121.105 Integer - number of goroutines
		105: func() interface{} {
			return int64(runtime.NumGoroutine())
		},
		/// 1.3.6.1.4.1.52535.121.110 Integer - number of command execution attempts
		110: func() interface{} {
			return int64(common.DurationStats.Count())
		},
		// 1.3.6.1.4.1.52535.121.111 Integer - number of web server requests processed
		111: func() interface{} {
			return int64(handler.DurationStats.Count())
		},
		// 1.3.6.1.4.1.52535.121.112 Integer - number of SMTP conversations
		112: func() interface{} {
			return int64(smtpd.DurationStats.Count())
		},
		// 1.3.6.1.4.1.52535.121.114 Integer - number of SMTP conversations
		114: func() interface{} {
			return int64(autounlock.UnlockStats.Count())
		},
		// 1.3.6.1.4.1.52535.121.115 Integer - size of outstanding mails to deliver in bytes
		115: func() interface{} {
			return int64(inet.OutstandingMailBytes)
		},
	}
)
