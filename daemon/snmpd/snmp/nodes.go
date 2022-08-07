package snmp

import (
	"encoding/asn1"
	"runtime"
	"sort"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
)

/*
ParentOID is the very top of OID hierarchy used in laitos SNMP server.
It contains a private enterprise number registered by Houzuo (Howard) Guo, 52535:
{iso(1) identified-organization(3) dod(6) internet(1) private(4) enterprise(1) 52535}

And laitos occupies number 121 underneath it.
*/
var ParentOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 52535, 121}

// OIDNodeFunc is a function that retrieves a latest system/application performance indicator value.
type OIDNodeFunc func() interface{}

var (
	// FirstOID is the very first OID among all supported nodes (OIDNodes).
	FirstOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 52535, 121, 100}
	// OIDNodes is a comprehensive list of suffix OIDs that correspond to nodes supported by laitos SNMP server.
	OIDNodes = map[int]OIDNodeFunc{
		// 1.3.6.1.4.1.52535.121.100 Octet string - public IP address
		100: func() interface{} {
			return []byte(inet.GetPublicIP())
		},
		// 1.3.6.1.4.1.52535.121.101 Integer - system clock - number of seconds since Unix epoch
		101: func() interface{} {
			return time.Now().In(time.UTC).Unix()
		},
		// 1.3.6.1.4.1.52535.121.102 Integer - number of seconds program has been running
		102: func() interface{} {
			return int64(time.Since(misc.StartupTime).Seconds())
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
			return int64(misc.CommandStats.Count())
		},
		// 1.3.6.1.4.1.52535.121.111 Integer - number of web server requests processed
		111: func() interface{} {
			return int64(misc.HTTPDStats.Count())
		},
		// 1.3.6.1.4.1.52535.121.112 Integer - number of SMTP conversations
		112: func() interface{} {
			return int64(misc.SMTPDStats.Count())
		},
		// 1.3.6.1.4.1.52535.121.114 Integer - number of SMTP conversations
		114: func() interface{} {
			return int64(misc.AutoUnlockStats.Count())
		},
		// 1.3.6.1.4.1.52535.121.115 Integer - size of outstanding mails to deliver in bytes
		115: func() interface{} {
			return int64(misc.OutstandingMailBytes)
		},
	}
	/*
		OIDSuffixList is a sorted list of suffix number among the OID nodes supported by laitos SNMP server. It is
		initialised from OIDNodes via package init function.
	*/
	OIDSuffixList []int
)

func init() {
	// Place all of the supported OID suffix numbers into a sorted list
	OIDSuffixList = make([]int, 0, len(OIDNodes))
	for suffix := range OIDNodes {
		OIDSuffixList = append(OIDSuffixList, suffix)
	}
	sort.Ints(OIDSuffixList)
}

// GetNode returns the calculation function for the input OID node, or false if the input OID is not supported (does not exist).
func GetNode(oid asn1.ObjectIdentifier) (nodeFun OIDNodeFunc, exists bool) {
	if len(oid) <= len(ParentOID) || !oid[:len(oid)-1].Equal(ParentOID) {
		return nil, false
	}
	nodeFun, exists = OIDNodes[oid[len(oid)-1]]
	return
}

// GetNextNode returns the OID subsequent to the input OID, and whether the input OID already is the very last one.
func GetNextNode(baseOID asn1.ObjectIdentifier) (asn1.ObjectIdentifier, bool) {
	if len(baseOID) <= len(ParentOID) || !baseOID[:len(baseOID)-1].Equal(ParentOID) {
		// Prefix is out of range
		return FirstOID, false
	}
	pos := sort.SearchInts(OIDSuffixList, baseOID[len(baseOID)-1])
	if pos == len(OIDSuffixList) {
		// Suffix is out of range
		return FirstOID, false
	} else if pos == len(OIDSuffixList)-1 {
		// The very last among all supported OIDs
		return baseOID, true
	}
	// Advance suffix to the next number
	baseOID[len(baseOID)-1] = OIDSuffixList[pos+1]
	return baseOID, false
}
