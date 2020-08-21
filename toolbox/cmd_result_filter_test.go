package toolbox

import (
	"net"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
)

func TestLintText_Transform(t *testing.T) {
	lint := LintText{}
	result := &Result{}
	if err := lint.Transform(result); err != nil || result.CombinedOutput != "" {
		t.Fatal(err, result.CombinedOutput)
	}

	// Retain original if all linting options are off
	mixedString := "abc \r\n\t def 123\t456 @#$<>\r\t\n 任意的"
	result.CombinedOutput = mixedString
	if err := lint.Transform(result); err != nil || result.CombinedOutput != mixedString {
		t.Fatal(err, result.CombinedOutput)
	}

	// Turn on all linting options
	lint.TrimSpaces = true
	lint.CompressToSingleLine = true
	lint.CompressSpaces = true
	lint.KeepVisible7BitCharOnly = true
	lint.BeginPosition = 2
	lint.MaxLength = 200

	// Even with all options turned on, linting an empty string should still result in an empty string.
	result.CombinedOutput = ""
	if err := lint.Transform(result); err != nil || result.CombinedOutput != "" {
		t.Fatal(err, result.CombinedOutput)
	}
	result.CombinedOutput = "aaa  \r\n\t"
	if err := lint.Transform(result); err != nil || result.CombinedOutput != "a" {
		t.Fatal(err, result.CombinedOutput)
	}
	result.CombinedOutput = mixedString
	if err := lint.Transform(result); err != nil || result.CombinedOutput != "c;def 123 456 @#$<>;???" {
		t.Fatal(err, result.CombinedOutput)
	}
	lint.MaxLength = 10
	if err := lint.Transform(result); err != nil || result.CombinedOutput != "def 123 45" {
		t.Fatal(err, result.CombinedOutput)
	}
}

func TestLintText_Transform_RetainLineBreaks(t *testing.T) {
	lint := LintText{}
	result := &Result{}
	if err := lint.Transform(result); err != nil || result.CombinedOutput != "" {
		t.Fatal(err, result.CombinedOutput)
	}

	mixedString := "abc \r\n\t def 123\t456 @#$<>\r\t\n 任意的"
	lint.TrimSpaces = true
	lint.CompressSpaces = true
	lint.KeepVisible7BitCharOnly = true
	lint.BeginPosition = 2
	lint.MaxLength = 200
	result.CombinedOutput = mixedString
	if err := lint.Transform(result); err != nil || result.CombinedOutput != "c\ndef 123 456 @#$<>\n???" {
		t.Fatal(err, result.CombinedOutput)
	}
}

func TestNotifyViaEmail_Transform(t *testing.T) {
	notify := NotifyViaEmail{}
	if notify.IsConfigured() {
		t.Fatal("should not be configured")
	}
	// It simply must not panic
	if err := notify.Transform(&Result{}); err != nil {
		t.Fatal(err)
	}

	notify.MailClient = inet.MailClient{
		MailFrom: "howard@localhost",
		MTAHost:  "localhost",
		MTAPort:  25,
	}
	notify.Recipients = []string{"howard@localhost"}
	if !notify.IsConfigured() {
		t.Fatal("should be configured now")
	}

	// It must not panic
	if err := notify.Transform(&Result{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)
	if _, err := net.Dial("tcp", "localhost:25"); err == nil {
		t.Log("Check howard@localhost mail box")
	} else {
		t.Log("MTA isn't running on localhost, observe an error message above.")
	}
}

func TestSayEmptyOutput_Transform(t *testing.T) {
	empty := SayEmptyOutput{}
	result := &Result{CombinedOutput: "    \t\r\n    "}
	if err := empty.Transform(result); err != nil || result.CombinedOutput != EmptyOutputText {
		t.Fatal(err, result)
	}
	result.CombinedOutput = "test"
	if err := empty.Transform(result); err != nil || result.CombinedOutput != "test" {
		t.Fatal(err, result)
	}
}
