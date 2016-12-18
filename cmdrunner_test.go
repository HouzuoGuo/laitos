package main

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestRemoveNonAscii(t *testing.T) {
	if o := RemoveNonAscii(""); o != "" {
		t.Fatal(o)
	} else if o := RemoveNonAscii("  a英漢字典b  "); o != "  a    b  " {
		t.Fatal(o)
	}
}

func TestLintOutput(t *testing.T) {
	if out := LintOutput(nil, "", 0, 0, false); out != emptyOutputResponse {
		t.Fatal([]byte(out))
		t.Fatal(out)
	}
	if out := LintOutput(nil, "0123456789", 0, 0, false); out != "0123456789" {
		t.Fatal(out)
	}
	if out := LintOutput(nil, "0123456789", 0, 10, false); out != "0123456789" {
		t.Fatal(out)
	}
	if out := LintOutput(nil, "0123456789abc", 0, 10, false); out != "0123456789" {
		t.Fatal(out)
	}
	if out := LintOutput(nil, "0123456789abc", 0, 10, true); out != "0123456789" {
		t.Fatal(out)
	}
	if out := LintOutput(errors.New("012345678"), "9", 0, 10, false); out != "012345678" {
		t.Fatal(out)
	}
	if out := LintOutput(errors.New("01234567"), "8", 0, 10, false); out != "01234567\n8" {
		t.Fatal(out)
	}
	if out := LintOutput(errors.New(" 0123456789 "), " 0123456789 ", 0, 0, false); out != "0123456789\n0123456789" {
		t.Fatal(out)
	}
	if out := LintOutput(errors.New(" 012345 \n 6789 "), " 012345 \n 6789 ", 0, 0, true); out != "012345;6789;012345;6789" {
		t.Fatal(out)
	}
	if out := LintOutput(errors.New(" 012345 \n 6789 "), " 012345 \n 6789 ", 0, 10, true); out != "012345;678" {
		t.Fatal(out)
	}
	utfSample := `S  (siemens)#1 S | 10 dS  (decisiemens);| 1000 mS  (millisiemens);| 0.001 kS  (kilosiemens);| 1×10^-9 abS  (absiemens);(unit officially deprecated);| 1×10^-9 emus of conductance;(unit officially deprecated);| 8.988×10^11 statS  (statsiemens);(unit officially deprecated);| 8.988×10^11 esus of conductance;(unit offic`
	if out := LintOutput(nil, utfSample, 0, 80, true); out != "S (siemens)#1 S | 10 dS (decisiemens);| 1000 mS (millisiemens);| 0.001 kS (kilos" {
		t.Fatal(out)
	}
	// Test hard output length limit
	if out := LintOutput(nil, strings.Repeat("12", maxOutputLen), 0, maxOutputLen*2, false); len(out) != maxOutputLen {
		t.Fatal(len(out))
	}
	if out := LintOutput(nil, strings.Repeat("12", maxOutputLen), 0, -1, false); len(out) != maxOutputLen {
		t.Fatal(len(out))
	}
	// Test seek position
	if out := LintOutput(nil, "0123456789", 1, 3, false); out != "123" {
		t.Fatal(out)
	}
	if out := LintOutput(nil, "0123456789", -1, 3, false); out != "012" {
		t.Fatal(out)
	}
	if out := LintOutput(nil, "0123456789", 3, 0, false); out != "3456789" {
		t.Fatal(out)
	}
	if out := LintOutput(nil, "0123456789", -1, -1, false); out != "0123456789" {
		t.Fatal(out)
	}
}

func TestCommandRunnerCheckConfig(t *testing.T) {
	run := CommandRunner{TimeoutSec: 10, TruncateLen: 100, PIN: "1234567"}
	if err := run.CheckConfig(); err != nil {
		t.Fatal(err)
	}

	run.TimeoutSec = 2
	if err := run.CheckConfig(); err == nil {
		t.Fatal("did not error")
	}
	run.TimeoutSec = 10

	run.TruncateLen = 8
	if err := run.CheckConfig(); err == nil {
		t.Fatal("did not error")
	}
	run.TruncateLen = 4000
	if err := run.CheckConfig(); err == nil {
		t.Fatal("did not error")
	}
	run.TruncateLen = 100

	run.PIN = ""
	if err := run.CheckConfig(); err == nil {
		t.Fatal("did not error")
	}
	run.PIN = "123456"
	if err := run.CheckConfig(); err == nil {
		t.Fatal("did not error")
	}
	run.PIN = "1234567"
	if err := run.CheckConfig(); err != nil {
		t.Fatal(err)
	}
}

func TestRunCommand(t *testing.T) {
	run := CommandRunner{SubHashSlashForPipe: false, TimeoutSec: 1, TruncateLen: 16}
	if out := run.RunCommand("echo a | grep a #/thisiscomment", false); out != "a" {
		t.Fatal(out)
	}
	if out := run.RunCommand("echo a && false # this is comment", false); out != "exit status 1\na" {
		t.Fatal(out)
	}
	if out := run.RunCommand("echo -e 'a\nb' > /proc/self/fd/1", false); out != "a\nb" {
		t.Fatal(out)
	}
	if out := run.RunCommand(`echo '"abc"' > /proc/self/fd/2`, false); out != `"abc"` {
		t.Fatal(out)
	}
	if out := run.RunCommand(`echo "'abc'"`, false); out != `'abc'` {
		t.Fatal(out)
	}
	if out := run.RunCommand(`sleep 2`, false); out != timeoutErr.Error() {
		t.Fatal(out)
	}
	if out := run.RunCommand(`echo 01234567891234567`, false); out != "0123456789123456" {
		t.Fatal(out)
	}
	if out := run.RunCommand("echo a #/ grep a", false); out != "a" {
		t.Fatal(out)
	}
	if out := run.RunCommand("echo a && false # this is comment", false); out != "exit status 1\na" {
		t.Fatal(out)
	}
	if out := run.RunCommand("echo -e 'a\nb' > /proc/self/fd/1", false); out != "a\nb" {
		t.Fatal(out)
	}
	if out := run.RunCommand(`echo '"abc"' > /proc/self/fd/2`, false); out != `"abc"` {
		t.Fatal(out)
	}
	if out := run.RunCommand(`echo "'abc'"`, false); out != `'abc'` {
		t.Fatal(out)
	}

	// Seek to position & override truncate length in output
	if out := run.RunCommand(magicSeekOutput+"echo01234567", false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicSeekOutput+"1 echo 01234567", false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicSeekOutput+"0 3 echo 01234567", false); out != "012" {
		t.Fatal(out)
	} else if out := run.RunCommand(magicSeekOutput+"3 3 echo 01234567", false); out != "345" {
		t.Fatal(out)
	} else if out := run.RunCommand(magicSeekOutput+"-1 -1 echo 01234567", false); out != "01234567" {
		t.Fatal(out)
	} else if out := run.RunCommand(magicSeekOutput+"    -1    -1    echo    01234567", false); out != "01234567" {
		t.Fatal(out)
	}

	// Substitution of pipe character
	run.SubHashSlashForPipe = true
	if out := run.RunCommand("echo a && false # this is comment", false); out != "exit status 1\na" {
		t.Fatal(out)
	}
	if out := run.RunCommand("yes #/ head -1", false); out != "y" {
		t.Fatal(out)
	}

	// Make sure process is indeed killed after timeout
	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	// The long-running process tries to delete the file
	if out := run.RunCommand(`sleep 2 && rm `+tmpFile.Name(), false); out != timeoutErr.Error() {
		t.Fatal(out)
	}
	if _, err := os.Stat(tmpFile.Name()); err != nil {
		t.Fatal(err)
	}
}

func TestFindCommand(t *testing.T) {
	run := CommandRunner{}
	if stmt := run.FindCommand("echo hi"); stmt != "" {
		t.Fatal(stmt)
	}

	run = CommandRunner{PIN: "abc123", PresetMessages: map[string]string{"": "echo hi"}}
	if stmt := run.FindCommand(""); stmt != "" {
		t.Fatal(stmt)
	}

	run = CommandRunner{PIN: "abc123"}
	if stmt := run.FindCommand("badpinhello world"); stmt != "" {
		t.Fatal(stmt)
	}
	if stmt := run.FindCommand("abc123hello world"); stmt != "hello world" {
		t.Fatal(stmt)
	}
	if stmt := run.FindCommand("   abc123    hello world   \r\n\t  "); stmt != "hello world" {
		t.Fatal(stmt)
	}

	run = CommandRunner{PIN: "irrelevant", PresetMessages: map[string]string{"abc": "123", "def": "456"}}
	if stmt := run.FindCommand("badbadbad"); stmt != "" {
		t.Fatal(stmt)
	}
	if stmt := run.FindCommand("   abcfoobar  "); stmt != "123" {
		t.Fatal(stmt)
	}
	if stmt := run.FindCommand("   deffoobar  "); stmt != "456" {
		t.Fatal(stmt)
	}
}

func TestRunBadParam(t *testing.T) {
	run := CommandRunner{}
	if out := run.RunCommand(magicWolframAlpha, false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicWolframAlpha+"a", false); out == paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicWolframAlpha+"    a    ", false); out == paramErr.Error() {
		t.Fatal(out)
	}

	if out := run.RunCommand(magicTwilioVoiceCall, false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwilioVoiceCall+"+49123", false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwilioVoiceCall+"+49123 a", false); out == paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwilioVoiceCall+"    +49123    a    ", false); out == paramErr.Error() {
		t.Fatal(out)
	}

	if out := run.RunCommand(magicTwilioSendSMS, false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwilioSendSMS+"+49123", false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwilioSendSMS+"+49123 a", false); out == paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwilioSendSMS+"    +49123    a    ", false); out == paramErr.Error() {
		t.Fatal(out)
	}

	if out := run.RunCommand(magicTwilioSendSMS, false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwilioSendSMS+"+49123", false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwilioSendSMS+"+49123 a", false); out == paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwilioSendSMS+"    +49123    a    ", false); out == paramErr.Error() {
		t.Fatal(out)
	}

	if out := run.RunCommand(magicTwitterGet, false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwitterGet+"1", false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwitterGet+"a b", false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwitterGet+"1 b", false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwitterGet+"1 2", false); out == paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwitterGet+"    1    2    ", false); out == paramErr.Error() {
		t.Fatal(out)
	}

	if out := run.RunCommand(magicTwitterPost, false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwitterPost+"a", false); out == paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicTwitterPost+"    a    ", false); out == paramErr.Error() {
		t.Fatal(out)
	}

	if out := run.RunCommand(magicFacebookPost, false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicFacebookPost+"a", false); out == paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand(magicFacebookPost+"    a    ", false); out == paramErr.Error() {
		t.Fatal(out)
	}

	if out := run.RunCommand("", false); out != paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand("cd", false); out == paramErr.Error() {
		t.Fatal(out)
	} else if out := run.RunCommand("    cd    ", false); out == paramErr.Error() {
		t.Fatal(out)
	}
}
