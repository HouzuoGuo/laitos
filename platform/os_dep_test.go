package platform

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
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

func TestInvokeProgram(t *testing.T) {
	if runtime.GOOS == "windows" {
		out, err := InvokeProgram([]string{"A=laitos123"}, 3, "hostname")
		if err != nil || len(out) < 1 {
			t.Fatal(err, out)
		}

		begin := time.Now()
		_, err = InvokeProgram(nil, 1, "cmd.exe", "/c", "waitfor dummydummy /t 60")
		if err == nil {
			t.Fatal("did not timeout")
		}
		duration := time.Since(begin)
		if duration > 3*time.Second {
			t.Fatal("did not kill before timeout")
		}

		// Verify cap on program output size
		out, err = InvokeProgram(nil, 3600, "cmd.exe", "/c", `type c:\windows\system32\ntoskrnl.exe`)
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != MaxExternalProgramOutputBytes {
			t.Fatal(len(out))
		}
	} else {
		out, err := InvokeProgram([]string{"A=laitos123"}, 3600, "printenv", "A")
		if err != nil || out != "laitos123\n" {
			t.Fatal(err, out)
		}

		begin := time.Now()
		_, err = InvokeProgram(nil, 1, "sleep", "5")
		if err == nil {
			t.Fatal("did not timeout")
		}
		duration := time.Since(begin)
		if duration > 3*time.Second {
			t.Fatal("did not kill before timeout")
		}

		// Verify cap on program output size
		out, err = InvokeProgram(nil, 1, "yes", "0123456789")
		if err == nil {
			t.Fatal("did not timeout")
		}
		if len(out) != MaxExternalProgramOutputBytes || !strings.Contains(out, "0123456789") {
			t.Fatal(len(out), !strings.Contains(out, "0123456789"))
		}
	}
}

func TestLockMemory(t *testing.T) {
	// just make sure it does not panic
	LockMemory()
}
