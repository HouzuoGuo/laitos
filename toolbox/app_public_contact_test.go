package toolbox

import (
	"reflect"
	"strings"
	"testing"
)

func TestContactSAR_Execute(t *testing.T) {
	sar := PublicContact{}
	if !sar.IsConfigured() {
		t.Fatal("no built-in SAR contact found")
	}
	if err := sar.SelfTest(); err == nil {
		t.Fatal("uninitialised should yield an error")
	}
	if err := sar.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := sar.SelfTest(); err != nil {
		t.Fatal(err)
	}

	fullOutput := strings.Join(sar.textRecords, "\n")
	if ret := sar.Execute(Command{Content: ""}); ret.Output != fullOutput {
		t.Fatal(ret)
	}
	if ret := sar.Execute(Command{Content: "this-does-not-exist"}); ret.Output != fullOutput {
		t.Fatal(ret)
	}

	mccOutput := `uk mcc +441343820902 ukmcc@hmcg.gov.uk
cn mcc +861065293298 cnmcc@mail.eastnet.com.cn
jp mcc +81335919000 op@kaiho.mlit.go.jp
`
	if ret := sar.Execute(Command{Content: "MCC"}); ret.Output != mccOutput {
		t.Fatal(ret)
	}

	if ret := sar.Execute(Command{Content: "MCC-and-something-extra"}); ret.Output != mccOutput {
		t.Fatal(ret)
	}
}

func TestGetAllSAREmails(t *testing.T) {
	mails := []string{
		"aid@cad.gov.hk",
		"hkmrcc@mardep.gov.hk",
		"ukarcc@hmcg.gov.uk",
		"ukmcc@hmcg.gov.uk",
		"rccaus@amsa.gov.au",
		"rccaus@amsa.gov.au",
		"jrcchalifax@sarnet.dnd.ca",
		"cnmcc@mail.eastnet.com.cn",
		"op@kaiho.mlit.go.jp",
		"jrccpgr@yen.gr",
		"mrcc@raja.fi",
		"odsmrcc@morflot.ru",
		"cnmrcc@mot.gov.cn",
		"rcc@mot.gov.il",
		"operations@jrcc-stavanger.no",
		"mrcckorea@korea.kr",
		"lantwatch@uscg.mil",
	}
	if ret := GetAllSAREmails(); !reflect.DeepEqual(ret, mails) {
		t.Fatal(ret)
	}
}
