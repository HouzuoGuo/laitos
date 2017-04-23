package env

import (
	"runtime"
	"testing"
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
