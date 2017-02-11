package feature

import (
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestShell_Execute(t *testing.T) {
	sh := Shell{}
	if !sh.IsConfigured() {
		t.Skip()
	}

	if err := sh.Initialise(); err != nil || sh.InterpreterPath == "" {
		t.Fatal(err, sh)
	}

	// Execute empty command
	ret := sh.Execute(&Command{TimeoutSec: 1, Content: "      "})
	if ret.Error != ErrEmptyCommand ||
		ret.ErrText() != ErrEmptyCommand.Error() ||
		ret.Output != "" ||
		ret.CombinedText() != ErrEmptyCommand.Error() {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Error, ret.ErrText(), ret.Output, ret.CombinedText())
	}

	// Execute a successful command
	ret = sh.Execute(&Command{TimeoutSec: 1, Content: `echo -n '"abc"' > /proc/self/fd/2`})
	if ret.Error != nil ||
		ret.ErrText() != "" ||
		ret.Output != `"abc"` ||
		ret.CombinedText() != `"abc"` {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Error, ret.ErrText(), ret.Output, ret.CombinedText())
	}

	// Execute a failing command
	ret = sh.Execute(&Command{TimeoutSec: 1, Content: `echo -e 'a\nb' && false # this is a comment`})
	if ret.Error == nil ||
		ret.ErrText() != "exit status 1" ||
		ret.Output != "a\nb\n" ||
		ret.CombinedText() != "exit status 1"+COMBINED_TEXT_SEP+"a\nb\n" {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Error, ret.ErrText(), ret.Output, ret.CombinedText())
	}

	// Execute a timeout command - it should not remove the temp file after timing out
	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	ret = sh.Execute(&Command{TimeoutSec: 2, Content: `echo -n abc && sleep 4 && rm ` + tmpFile.Name()})
	if ret.Error != ErrExecTimeout ||
		ret.ErrText() != ErrExecTimeout.Error() ||
		ret.Output != "abc" ||
		ret.CombinedText() != ErrExecTimeout.Error()+COMBINED_TEXT_SEP+"abc" {
		t.Fatalf("%v\n%s\n%s\n%s", ret.Error, ret.ErrText(), ret.Output, ret.CombinedText())
	}
	// If the command was truly killed, the file would still remain.
	time.Sleep(3 * time.Second)
	if _, err := os.Stat(tmpFile.Name()); err != nil {
		t.Fatal(err)
	}
}
