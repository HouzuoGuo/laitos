package feature

import (
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestShell(t *testing.T) {
	sh := Shell{TimeoutSec: SHELL_TIMEOUT_SEC_MIN - 1}
	if err := sh.InitAndTest(); err == nil {
		t.Fatal("did not error")
	}
	sh.TimeoutSec = SHELL_TIMEOUT_SEC_MAX + 1
	if err := sh.InitAndTest(); err == nil {
		t.Fatal("did not error")
	}
	sh.TimeoutSec = 2
	if err := sh.InitAndTest(); err != nil || sh.InterpreterPath == "" {
		t.Fatal(err, sh)
	}

	// Execute empty command
	ret := sh.Execute("      ")
	if ret.Err() != ErrShellCommandEmpty ||
		ret.ErrText() != ErrShellCommandEmpty.Error() ||
		ret.OutText() != "" ||
		ret.CombinedText() != ErrShellCommandEmpty.Error() {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Err(), ret.ErrText(), ret.OutText(), ret.CombinedText())
	}

	// Execute a successful command
	ret = sh.Execute(`echo -n '"abc"' > /proc/self/fd/2`)
	if ret.Err() != nil ||
		ret.ErrText() != "" ||
		ret.OutText() != `"abc"` ||
		ret.CombinedText() != `"abc"` {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Err(), ret.ErrText(), ret.OutText(), ret.CombinedText())
	}

	// Execute a failing command
	ret = sh.Execute(`echo -e 'a\nb' && false # this is a comment`)
	if ret.Err() == nil ||
		ret.ErrText() != "exit status 1" ||
		ret.OutText() != "a\nb\n" ||
		ret.CombinedText() != "exit status 1"+SHELL_COMBINED_TEXT_SEP+"a\nb\n" {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Err(), ret.ErrText(), ret.OutText(), ret.CombinedText())
	}

	// Execute a timeout command - it should not remove the temp file after timing out
	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	ret = sh.Execute(`echo -n abc && sleep 4 && rm ` + tmpFile.Name())
	if ret.Err() != ErrShellTimeout ||
		ret.ErrText() != ErrShellTimeout.Error() ||
		ret.OutText() != "abc" ||
		ret.CombinedText() != ErrShellTimeout.Error()+SHELL_COMBINED_TEXT_SEP+"abc" {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Err(), ret.ErrText(), ret.OutText(), ret.CombinedText())
	}
	// If the command was truly killed, the file would still remain.
	time.Sleep(3 * time.Second)
	if _, err := os.Stat(tmpFile.Name()); err != nil {
		t.Fatal(err)
	}

	// Execute a command that produces excessive output
	ret = sh.Execute(`printf %` + strconv.Itoa(SHELL_RAW_OUTPUT_LEN_MAX*2) + `s | tr ' ' '+'`)
	expected := strings.Repeat("+", SHELL_RAW_OUTPUT_LEN_MAX)
	if ret.Err() != nil ||
		ret.ErrText() != "" ||
		ret.OutText() != expected ||
		ret.CombinedText() != expected {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Err(), ret.ErrText(), ret.OutText(), ret.CombinedText())
	}

	// Execute a failing command that produces excessive output
	ret = sh.Execute(`printf %` + strconv.Itoa(SHELL_RAW_OUTPUT_LEN_MAX*2) + `s | tr ' ' '+' && false`)
	expectedCombined := "exit status 1" + SHELL_COMBINED_TEXT_SEP + expected
	if ret.Err() == nil ||
		ret.ErrText() != "exit status 1" ||
		ret.OutText() != expected ||
		ret.CombinedText() != expectedCombined {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Err(), ret.ErrText(), ret.OutText(), ret.CombinedText())
	}
}
