package feature

import (
	"github.com/HouzuoGuo/laitos/lalog"
	"strings"
	"testing"
)

func TestEnvInfo_Execute(t *testing.T) {
	info := EnvInfo{}
	if !info.IsConfigured() {
		t.Fatal("not configured")
	}
	if err := info.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := info.SelfTest(); err != nil {
		t.Fatal(err)
	}
	if ret := info.Execute(Command{Content: "wrong"}); ret.Error != ErrBadEnvInfoChoice {
		t.Fatal(ret)
	}
	if ret := info.Execute(Command{Content: "runtime"}); ret.Error != nil || strings.Index(ret.Output, "Public IP") == -1 {
		t.Fatal(ret)
	}
	logger := lalog.Logger{}
	logger.Printf("envinfo test", "", nil, "")
	if ret := info.Execute(Command{Content: "log"}); ret.Error != nil || strings.Index(ret.Output, "envinfo test") == -1 {
		t.Fatal(ret)
	}
	if ret := info.Execute(Command{Content: "stack"}); ret.Error != nil || strings.Index(ret.Output, "routine") == -1 {
		t.Fatal(ret)
	}
}
