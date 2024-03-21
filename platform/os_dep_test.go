package platform

import (
	"os"
	"runtime"
	"testing"
)

func TestGetRootDiskUsageKB(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getenv("CIRCLECI") != "" {
		// Just make sure the function does not crash
		GetRootDiskUsageKB()
		return
	}
	used, free, total := GetRootDiskUsageKB()
	if used == 0 || free == 0 || total == 0 || used+free != total {
		t.Fatal(used/1024, free/1024, total/1024)
	}
}
func TestLockMemory(t *testing.T) {
	// Just make sure it won't panic.
	LockMemory()
}

func TestSync(t *testing.T) {
	// Just make sure it won't panic.
	Sync()
}
