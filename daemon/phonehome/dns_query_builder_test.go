package phonehome

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/toolbox"
)

func TestEncodeToDTMF(t *testing.T) {
	// Only letters, only numbers, and only symbols.
	if s := EncodeToDTMF("abcdABCD"); s != "abcdABCD" {
		t.Fatal(s)
	}
	if s := EncodeToDTMF("1230123"); s != "11012013010110120130" {
		t.Fatal(s)
	}
	if s := EncodeToDTMF(`@~}.`); s != "1120122013201420" {
		t.Fatal(s)
	}
	// Mix of everything
	if s := EncodeToDTMF("A \nb "); s != "A01470b0" {
		t.Fatal(s)
	}
	s := EncodeToDTMF(fmt.Sprintf("a1!%c 2", toolbox.SubjectReportSerialisedFieldSeparator))
	if s != "a110111014600120" {
		t.Fatal(s)
	}
}

func TestGetDNSQuery(t *testing.T) {
	// Short queries
	if q := GetDNSQuery("", ""); q != "_." {
		t.Fatal(q)
	}
	if q := GetDNSQuery("abc", ""); q != "_.abc." {
		t.Fatal(q)
	}
	if q := GetDNSQuery("abc", "def.ghi"); q != "_.abc.def.ghi" {
		t.Fatal(q)
	}
	if q := GetDNSQuery("", "def.ghi"); q != "_.def.ghi" {
		t.Fatal(q)
	}
	if q := GetDNSQuery("a1!2B", "def.ghi"); q != "_.a1101110120B.def.ghi" {
		t.Fatal(q)
	}
	if q := GetDNSQuery("a\nB", "def.ghi"); q != "_.a1470B.def.ghi" {
		t.Fatal(q)
	}
	// Span across several labels
	q := GetDNSQuery(strings.Repeat("abcdefghijklmnopqrstuvwxyz", 100), "example.com")
	if q != "_.abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefgh.ijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnop.qrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwx.yzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwx.example.com" {
		t.Fatal(q)
	}
	// Sophisticated query
	req := toolbox.SubjectReportRequest{
		SubjectIP:       "1.2.3.4",
		SubjectHostName: "hzgl-dev",
		SubjectPlatform: "windows",
		SubjectComment:  "comment 1\ncomment 2",
		CommandRequest: toolbox.AppCommandRequest{
			Command: "pass.s date",
		},
		CommandResponse: toolbox.AppCommandResponse{
			Command:        "pass.s date",
			ReceivedAt:     time.Unix(1234567890, 0),
			Result:         "result 1\nresult2",
			RunDurationSec: 321,
		},
	}
	q = GetDNSQuery("987654987654"+toolbox.StoreAndForwardMessageProcessorTrigger+req.SerialiseCompact(), "example.com")
	if q != "_.190180170160150140190180170160150140142010mhzgl1240dev1460pa.ss1420s0date1460pass1420s0date1460result01101470result120146.0windows1460comment01101470comment01201460110142012014201301.4201401460110120130140150160170180190101460130120110.example.com" {
		t.Fatal(q)
	}
}
