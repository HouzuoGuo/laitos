package platform

import (
	"strings"
	"testing"
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
