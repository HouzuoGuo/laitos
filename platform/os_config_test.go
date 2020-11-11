package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
)

func TestGetProgramMemUsageKB(t *testing.T) {
	if runtime.GOOS != "linux" {
		// Just make sure the function does not crash
		GetProgramMemoryUsageKB()
		return
	}
	if usage := GetProgramMemoryUsageKB(); usage < 1000 {
		t.Fatal(usage)
	}
}

func TestGetSystemMemoryUsageKB(t *testing.T) {
	if runtime.GOOS != "linux" {
		// Just make sure the function does not crash
		GetSystemMemoryUsageKB()
		return
	}
	used, total := GetSystemMemoryUsageKB()
	if used < 1000 || total < used {
		t.Fatal(used, total)
	}
}

func TestGetSystemLoad(t *testing.T) {
	if runtime.GOOS != "linux" {
		// Just make sure the function does not crash
		GetSystemMemoryUsageKB()
		return
	}
	load := GetSystemLoad()
	if len(load) < 6 {
		t.Fatal(load)
	}
}

func TestGetSystemUptimeSec(t *testing.T) {
	if runtime.GOOS != "linux" {
		// Just make sure the function does not crash
		GetSystemUptimeSec()
		return
	}
	uptime := GetSystemUptimeSec()
	if uptime < 10 {
		t.Fatal(uptime)
	}
}
func TestGetSysctl(t *testing.T) {
	key := "kernel.pid_max"
	if runtime.GOOS != "linux" {
		// Just make sure the function does not crash
		_, _ = GetSysctlInt(key)
		_, _ = GetSysctlStr(key)
		return
	}
	if val, err := GetSysctlStr(key); err != nil || val == "" {
		t.Fatal(val, err)
	}
	if val, err := GetSysctlInt(key); err != nil || val < 1 {
		t.Fatal(val, err)
	}
	if old, err := IncreaseSysctlInt(key, 65535); old == 0 ||
		(err != nil && !strings.Contains(err.Error(), "permission") && !strings.Contains(err.Error(), "read-only")) {
		t.Fatal(err)
	}
}

func TestCopyNonEssentialUtilities(t *testing.T) {
	CopyNonEssentialUtilities(lalog.Logger{})
	if HostIsWindows() {
		// Just make sure it does not panic
		return
	}
	for _, utilName := range []string{"busybox", "toybox", "phantomjs"} {
		if _, err := os.Stat(filepath.Join(UtilityDir, utilName)); err != nil {
			t.Fatal("cannot find program "+utilName, err)
		}
	}
}

func TestGetLocalUserNames(t *testing.T) {
	names := GetLocalUserNames()
	if len(names) < 2 || (!names["root"] && !names["Administrator"]) {
		t.Fatal(names)
	}
}

func TestBlockUserLogin(t *testing.T) {
	// just make sure it does not panic
	t.Log(BlockUserLogin("nobody"))
}

func TestDisableStopDaemon(t *testing.T) {
	// just make sure it does not panic
	t.Log(DisableStopDaemon("this-service-does-not-exist"))
}

func TestEnableStartDaemon(t *testing.T) {
	// just make sure it does not panic
	t.Log(EnableStartDaemon("this-service-does-not-exist"))
}

func TestDisableInterferingResolved(t *testing.T) {
	// just make sure it does not panic
	t.Log(DisableInterferingResolved())
}

func TestSwapOff(t *testing.T) {
	// just make sure it does not panic
	_ = SwapOff()
}

func TestSetTimeZone(t *testing.T) {
	// just make sure it does not panic
	t.Log(SetTimeZone("UTC"))
}

func TestGetSysSummary(t *testing.T) {
	txt := GetProgramStatusSummary(true)
	hostName, _ := os.Hostname()
	if !strings.Contains(txt, inet.GetPublicIP()) || !strings.Contains(txt, hostName) || !strings.Contains(txt, os.Environ()[0]) {
		t.Fatal(txt)
	}
}
