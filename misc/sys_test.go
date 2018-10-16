package misc

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/platform"
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
		GetSysctlInt(key)
		GetSysctlStr(key)
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

func TestInvokeShell(t *testing.T) {
	if HostIsWindows() {
		out, err := InvokeShell(3, "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe", "echo $env:windir")
		if err != nil || !strings.Contains(strings.ToLower(out), "windows") {
			t.Fatal(err, out)
		}
	} else {
		out, err := InvokeShell(1, "/bin/sh", "echo $PATH")
		if err != nil || out != platform.CommonPATH+"\n" {
			t.Fatal(err, out)
		}
	}
}

func TestPrepareUtilities(t *testing.T) {
	PrepareUtilities(lalog.Logger{})
	if HostIsWindows() {
		// Just make sure it does not panic
		return
	}
	for _, utilName := range []string{"busybox", "toybox", "phantomjs"} {
		if _, err := os.Stat(filepath.Join(platform.UtilityDir, utilName)); err != nil {
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

func TestDisableInterferingResolvd(t *testing.T) {
	// just make sure it does not panic
	t.Log(DisableInterferingResolved())
}

func TestSwapOff(t *testing.T) {
	// just make sure it does not panic
	SwapOff()
}

func TestSetTimeZone(t *testing.T) {
	// just make sure it does not panic
	t.Log(SetTimeZone("UTC"))
}
