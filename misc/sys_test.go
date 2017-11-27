package misc

import (
	"runtime"
	"strings"
	"testing"
	"time"
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

func TestInvokeProgram(t *testing.T) {
	out, err := InvokeProgram([]string{"A=laitos123"}, 10, "printenv", "A")
	if err != nil || out != "laitos123\n" {
		t.Fatal(err, out)
	}

	begin := time.Now()
	out, err = InvokeProgram(nil, 1, "sleep", "5")
	if err == nil {
		t.Fatal("did not timeout")
	}
	duration := time.Now().Unix() - begin.Unix()
	if duration > 2 {
		t.Fatal("did not kill before timeout")
	}
}

func TestInvokeShell(t *testing.T) {
	out, err := InvokeShell(1, "/bin/bash", "printenv PATH")
	if err != nil || out != CommonPATH+"\n" {
		t.Fatal(err, out)
	}
}
