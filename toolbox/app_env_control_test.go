package toolbox

import (
	"fmt"
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

func TestEnvControl_Execute(t *testing.T) {
	info := EnvControl{}
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
	if ret := info.Execute(Command{Content: "info"}); ret.Error != nil || !strings.Contains(ret.Output, "Sys/prog uptime") {
		t.Fatal(ret)
	}
	// Test log retrieval
	logger := lalog.Logger{}
	logger.Info("envinfo printf test", "", nil, "")
	logger.Warning("envinfo warningf test", "", nil, "")
	if ret := info.Execute(Command{Content: "log"}); ret.Error != nil || !strings.Contains(ret.Output, "envinfo printf test") {
		t.Fatal(ret)
	}
	if ret := info.Execute(Command{Content: "warn"}); ret.Error != nil || !strings.Contains(ret.Output, "envinfo warningf test") {
		t.Fatal(ret)
	}
	// Test stack retrieval
	if ret := info.Execute(Command{Content: "stack"}); ret.Error != nil || !strings.Contains(ret.Output, "routine") {
		t.Fatal(ret)
	}
	// Test system tuning
	ret := info.Execute(Command{Content: "tune"})
	fmt.Println(ret.Output)
	if ret.Error != nil {
		t.Fatal(ret)
	}
	// Test lockdown
	if ret := info.Execute(Command{Content: "lock"}); !strings.Contains(ret.Output, "OK") {
		t.Fatal(ret)
	}
	if !misc.EmergencyLockDown {
		t.Fatal("did not lockdown")
	}
	misc.EmergencyLockDown = false
}
