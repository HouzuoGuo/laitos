package platform

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

func TestInvokeShell(t *testing.T) {
	if HostIsWindows() {
		out, err := InvokeShell(3, "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe", "echo $env:windir")
		if err != nil || !strings.Contains(strings.ToLower(out), "windows") {
			t.Fatal(err, out)
		}
	} else {
		out, err := InvokeShell(1, "/bin/sh", "echo $PATH")
		if err != nil || out != CommonPATH+"\n" {
			t.Fatal(err, out)
		}
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

func TestStartProgramTermination(t *testing.T) {
	if runtime.GOOS == "windows" {
		begin := time.Now()
		termChan := make(chan struct{})
		startChan := make(chan error)
		errChan := make(chan error)
		go func() {
			errChan <- StartProgram(nil, 100, lalog.DiscardCloser, lalog.DiscardCloser, startChan, termChan, "sleep", "10")
		}()
		<-startChan
		close(termChan)
		duration := time.Since(begin)
		if duration > 1*time.Second {
			t.Fatal("failed to terminate external program in time")
		}
		if err := <-errChan; err == nil {
			t.Fatal("did not terminate with an abnormal exit code")
		}
	} else {
		begin := time.Now()
		termChan := make(chan struct{})
		startChan := make(chan error)
		errChan := make(chan error)
		go func() {
			errChan <- StartProgram(nil, 100, lalog.DiscardCloser, lalog.DiscardCloser, startChan, termChan, "sleep", "10")
		}()
		<-startChan
		close(termChan)
		duration := time.Since(begin)
		if duration > 1*time.Second {
			t.Fatal("failed to terminate external program in time")
		}
		if err := <-errChan; err == nil {
			t.Fatal("did not terminate with an abnormal exit code")
		}
	}
}
