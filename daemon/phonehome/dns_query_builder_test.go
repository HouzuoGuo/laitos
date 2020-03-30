package phonehome

import (
	"fmt"
	"strings"
	"testing"

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
	// Span across several labels
	q := GetDNSQuery(strings.Repeat("abcdefghijklmnopqrstuvwxyz", 100), "example.com")
	if q != "_.abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefgh.ijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnop.qrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwx.yzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwx.example.com" {
		t.Fatal(q)
	}
}
