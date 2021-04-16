package toolbox

import (
	"context"
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
	if ret := sar.Execute(context.Background(), Command{Content: ""}); ret.Output != fullOutput {
		t.Fatal(ret)
	}
	if ret := sar.Execute(context.Background(), Command{Content: "zzz-this-does-not-exist"}); ret.Output != fullOutput {
		t.Fatal(ret)
	}

	mccOutput := `cn mcc +861065293298 cnmcc@cttic.cn
jp mcc +81335919000 op@kaiho.mlit.go.jp
uk mcc +443443820902 ukmcc@hmcg.gov.uk
`
	if ret := sar.Execute(context.Background(), Command{Content: "MCC"}); ret.Output != mccOutput {
		t.Fatal(ret)
	}

	if ret := sar.Execute(context.Background(), Command{Content: "MCC-and-something-extra"}); ret.Output != mccOutput {
		t.Fatal(ret)
	}
}

func TestGetAllSAREmails(t *testing.T) {
	mails := []string{
		"aid@cad.gov.hk",
		"hkmrcc@mardep.gov.hk",
		"rccaus@amsa.gov.au",
		"rccaus@amsa.gov.au",
		"jrcchalifax@sarnet.dnd.ca",
		"cnmcc@cttic.cn",
		"op@kaiho.mlit.go.jp",
		"jrccpgr@yen.gr",
		"ukmcc@hmcg.gov.uk",
		"mrcc@raja.fi",
		"odsmrcc@morflot.ru",
		"cnmrcc@mot.gov.cn",
		"rcc@mot.gov.il",
		"operations@jrcc-stavanger.no",
		"mrcckorea@korea.kr",
		"falmouthcoastguard@mcga.gov.uk",
		"lantwatch@uscg.mil",
	}
	if ret := GetAllSAREmails(); !reflect.DeepEqual(ret, mails) {
		t.Fatal(ret)
	}

}
