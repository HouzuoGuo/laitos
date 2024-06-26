package toolbox

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/platform"
)

func TestShell_WindowsExecute(t *testing.T) {
	if !platform.HostIsWindows() {
		t.Skip("this test is only applicable on Windows")
	}
	sh := Shell{Unrestricted: true}
	if !sh.IsConfigured() {
		t.Fatal("should be configured")
	}
	if err := sh.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := sh.SelfTest(); err != nil {
		t.Fatal(err)
	}

	// Execute empty command
	ret := sh.Execute(context.Background(), Command{TimeoutSec: 1, Content: "      "})
	if ret.Error != ErrEmptyCommand ||
		ret.ErrText() != ErrEmptyCommand.Error() ||
		ret.Output != "" ||
		ret.ResetCombinedText() != ErrEmptyCommand.Error() {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Error, ret.ErrText(), ret.Output, ret.ResetCombinedText())
	}

	// Execute a successful command
	ret = sh.Execute(context.Background(), Command{TimeoutSec: 3, Content: `echo '"abc"'`})
	if ret.Error != nil ||
		ret.ErrText() != "" ||
		strings.TrimSpace(ret.Output) != `"abc"` ||
		strings.TrimSpace(ret.ResetCombinedText()) != `"abc"` {
		t.Fatalf("Err: %v\nErrText: %s\nOutput: %s\nCombinedOutput: %s", ret.Error, ret.ErrText(), ret.Output, ret.ResetCombinedText())
	}

	// Execute a failing command
	ret = sh.Execute(context.Background(), Command{TimeoutSec: 3, Content: `does-not-exist`})
	if ret.Error == nil ||
		ret.ErrText() != "exit status 1" ||
		!strings.Contains(ret.Output, "CommandNotFoundException") ||
		!strings.Contains(ret.ResetCombinedText(), "CommandNotFoundException") {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Error, ret.ErrText(), ret.Output, ret.ResetCombinedText())
	}
}

func TestShell_RestrictedShell(t *testing.T) {
	sh := Shell{}
	if err := sh.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := sh.SelfTest(); err != nil {
		t.Fatal(err)
	}
	ret := sh.Execute(context.Background(), Command{TimeoutSec: 1, Content: "shutdown"})
	if ret.Error != ErrRestrictedShell || ret.Output != "" {
		t.Fatalf("%+v", ret)
	}
	ret = sh.Execute(context.Background(), Command{TimeoutSec: 1, Content: "date"})
	if ret.Error != nil || len(ret.Output) < 10 {
		t.Fatalf("%+v", ret)
	}
}

func TestShell_NonWindowsExecute(t *testing.T) {
	if platform.HostIsWindows() {
		t.Skip("this test is skipped on Windows")
	}
	sh := Shell{Unrestricted: true}
	if !sh.IsConfigured() {
		t.Fatal("should be configured")
	}
	if err := sh.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := sh.SelfTest(); err != nil {
		t.Fatal(err)
	}

	t.Run("execute an empty command", func(t *testing.T) {
		ret := sh.Execute(context.Background(), Command{TimeoutSec: 1, Content: "      "})
		if ret.Error != ErrEmptyCommand ||
			ret.ErrText() != ErrEmptyCommand.Error() ||
			ret.Output != "" ||
			ret.ResetCombinedText() != ErrEmptyCommand.Error() {
			t.Fatalf("%v\n%s\n%s\n%s", ret.Error, ret.ErrText(), ret.Output, ret.ResetCombinedText())
		}
	})

	t.Run("execute a succeesful command", func(t *testing.T) {
		ret := sh.Execute(context.Background(), Command{TimeoutSec: 1, Content: `echo -n '"abc"' >&2`})
		if ret.Error != nil ||
			ret.ErrText() != "" ||
			ret.Output != `"abc"` ||
			ret.ResetCombinedText() != `"abc"` {
			t.Fatalf("Err: %v\nErrText: %s\nOutput: %s\nCombinedOutput: %s", ret.Error, ret.ErrText(), ret.Output, ret.ResetCombinedText())
		}
	})

	t.Run("execute a failing command", func(t *testing.T) {
		ret := sh.Execute(context.Background(), Command{TimeoutSec: 1, Content: `echo -e 'a\nb' && false # this is a comment`})
		if ret.Error == nil ||
			ret.ErrText() != "exit status 1" ||
			ret.Output != "a\nb\n" ||
			ret.ResetCombinedText() != "exit status 1"+CombinedTextSeparator+"a\nb\n" {
			t.Fatalf("%v\n%s\n%s\n%s", ret.Error, ret.ErrText(), ret.Output, ret.ResetCombinedText())
		}
	})

	t.Run("timing out a command", func(t *testing.T) {
		// The command should time out before deleting this temp file.
		tmpFile, err := os.CreateTemp("", "")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())
		start := time.Now()
		ret := sh.Execute(context.Background(), Command{TimeoutSec: 2, Content: `echo -n abc; sleep 5; rm ` + tmpFile.Name()})
		if !strings.Contains(ret.Error.Error(), "terminated") ||
			ret.Output != "abc" ||
			!strings.Contains(ret.ResetCombinedText(), "terminated") || !strings.Contains(ret.ResetCombinedText(), CombinedTextSeparator+"abc") {
			t.Fatalf("%v\n%s\n%s\n%s", ret.Error, ret.ErrText(), ret.Output, ret.ResetCombinedText())
		}
		if elapsed := time.Since(start); elapsed > 4*time.Second {
			t.Fatalf("did not kill in time, elapsed %v", elapsed)
		}
		// If the command was truly killed, the file would still remain.
		time.Sleep(6 * time.Second)
		if _, err := os.Stat(tmpFile.Name()); err != nil {
			t.Fatal(err)
		}
	})
}
