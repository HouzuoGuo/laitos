package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
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
	for _, utilName := range []string{"busybox", "toybox"} {
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
	summary := GetProgramStatusSummary(true)
	hostName, _ := os.Hostname()
	if summary.HostName != hostName ||
		summary.PublicIP != inet.GetPublicIP().String() ||
		summary.PID == 0 || summary.PPID == 0 ||
		summary.ExePath == "" || summary.WorkingDirPath == "" ||
		time.Since(summary.ClockTime).Seconds() > 3 {
		t.Fatalf("%+v", summary)
	}
	t.Logf("%s", summary)
}

func TestGetRedactedEnviron(t *testing.T) {
	for _, keyToRedact := range []string{"AWS_SESSION_TOKEN", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", misc.EnvironmentDecryptionPassword} {
		os.Setenv(keyToRedact, "TestGetRedactedEnviron")
	}
	envKeyValue := make(map[string]string)
	for _, keyValue := range GetRedactedEnviron() {
		fields := strings.SplitN(keyValue, "=", 2)
		envKeyValue[fields[0]] = fields[1]
		fmt.Println(keyValue)
	}
	for _, keyToRedact := range []string{"AWS_SESSION_TOKEN", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", misc.EnvironmentDecryptionPassword} {
		if val := envKeyValue[keyToRedact]; val != "REDACTED" {
			t.Fatalf("did not redact %s=%s", keyToRedact, val)
		}
	}
	for _, key := range []string{"HOME", "PATH", "PWD"} {
		if val := envKeyValue[key]; val == "" || val == "REDACTED" {
			t.Fatalf("ordinary key went missing %s=%s", key, val)
		}
	}
}

func TestFindNumInRegexGroup(t *testing.T) {
	regex := regexp.MustCompile(`^prefix([\d]+)m([\d]+)suffix$`)
	var tests = []struct {
		inputString string
		groupNum    int
		expected    int64
	}{
		{"not-a-match", 0, 0},
		{"not-a-match", 1, 0},
		{"not-a-match", 2, 0},
		{"prefix123m456suffix", 0, 0},
		{"prefix123m456suffix", 1, 123},
		{"prefix123m456suffix", 2, 456},
		{"prefix123m456suffix", 3, 0},
	}
	for _, test := range tests {
		if num := FindNumInRegexGroup(regex, test.inputString, test.groupNum); num != test.expected {
			t.Fatalf("Input: %s, group num: %d, expected: %d, actual: %d", test.inputString, test.groupNum, test.expected, num)
		}
	}
}

func TestGetDefaultShellInterpreter(t *testing.T) {
	shell := GetDefaultShellInterpreter()
	cmd := exec.Command(shell)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}
