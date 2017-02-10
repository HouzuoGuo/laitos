package feature

import (
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestShell(t *testing.T) {
	sh := Shell{}
	if err := sh.InitAndTest(); err != nil || sh.InterpreterPath == "" {
		t.Fatal(err, sh)
	}

	// Execute empty command
	ret := sh.Execute(1, "      ")
	if ret.Err() != ErrEmptyCommand ||
		ret.ErrText() != ErrEmptyCommand.Error() ||
		ret.OutText() != "" ||
		ret.CombinedText() != ErrEmptyCommand.Error() {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Err(), ret.ErrText(), ret.OutText(), ret.CombinedText())
	}

	// Execute a successful command
	ret = sh.Execute(1, `echo -n '"abc"' > /proc/self/fd/2`)
	if ret.Err() != nil ||
		ret.ErrText() != "" ||
		ret.OutText() != `"abc"` ||
		ret.CombinedText() != `"abc"` {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Err(), ret.ErrText(), ret.OutText(), ret.CombinedText())
	}

	// Execute a failing command
	ret = sh.Execute(1, `echo -e 'a\nb' && false # this is a comment`)
	if ret.Err() == nil ||
		ret.ErrText() != "exit status 1" ||
		ret.OutText() != "a\nb\n" ||
		ret.CombinedText() != "exit status 1"+COMBINED_TEXT_SEP+"a\nb\n" {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Err(), ret.ErrText(), ret.OutText(), ret.CombinedText())
	}

	// Execute a timeout command - it should not remove the temp file after timing out
	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	ret = sh.Execute(2, `echo -n abc && sleep 4 && rm `+tmpFile.Name())
	if ret.Err() != ErrExecTimeout ||
		ret.ErrText() != ErrExecTimeout.Error() ||
		ret.OutText() != "abc" ||
		ret.CombinedText() != ErrExecTimeout.Error()+COMBINED_TEXT_SEP+"abc" {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Err(), ret.ErrText(), ret.OutText(), ret.CombinedText())
	}
	// If the command was truly killed, the file would still remain.
	time.Sleep(3 * time.Second)
	if _, err := os.Stat(tmpFile.Name()); err != nil {
		t.Fatal(err)
	}
}
