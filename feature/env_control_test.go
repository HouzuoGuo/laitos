package feature

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/global"
	"strings"
	"testing"
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
	if ret := info.Execute(Command{Content: "info"}); ret.Error != nil || strings.Index(ret.Output, "Sys/prog uptime") == -1 {
		t.Fatal(ret)
	}
	// Test log retrieval
	logger := global.Logger{}
	logger.Printf("envinfo printf test", "", nil, "")
	logger.Warningf("envinfo warningf test", "", nil, "")
	if ret := info.Execute(Command{Content: "log"}); ret.Error != nil || strings.Index(ret.Output, "envinfo printf test") == -1 {
		t.Fatal(ret)
	}
	if ret := info.Execute(Command{Content: "warn"}); ret.Error != nil || strings.Index(ret.Output, "envinfo warningf test") == -1 {
		t.Fatal(ret)
	}
	// Test stack retrieval
	if ret := info.Execute(Command{Content: "stack"}); ret.Error != nil || strings.Index(ret.Output, "routine") == -1 {
		t.Fatal(ret)
	}
	// Test system tuning
	ret := info.Execute(Command{Content: "tune"})
	fmt.Println(ret.Output)
	if ret.Error != nil {
		t.Fatal(ret)
	}
	// Test lockdown
	if ret := info.Execute(Command{Content: "lock"}); strings.Index(ret.Output, "OK") == -1 {
		t.Fatal(ret)
	}
	if !global.EmergencyLockDown {
		t.Fatal("did not lockdown")
	}
	global.EmergencyLockDown = false
}
