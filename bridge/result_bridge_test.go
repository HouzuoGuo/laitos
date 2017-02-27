package bridge

import (
	"errors"
	"github.com/HouzuoGuo/websh/email"
	"github.com/HouzuoGuo/websh/feature"
	"testing"
)

func TestLintText_Transform(t *testing.T) {
	lint := LintText{}
	result := &feature.Result{}
	if err := lint.Transform(result); err != nil || result.CombinedOutput != "" {
		t.Fatal(err, result.CombinedOutput)
	}

	mixedString := "abc  def 123 \r\t\n @#$<>\r\t\n 任意的"
	result.CombinedOutput = mixedString
	if err := lint.Transform(result); err != nil || result.CombinedOutput != mixedString {
		t.Fatal(err, result.CombinedOutput)
	}

	// Even with all options turned on, linting an empty string should still result in an empty string.
	lint.TrimSpaces = true
	lint.CompressToSingleLine = true
	lint.KeepVisible7BitCharOnly = true
	lint.CompressSpaces = true
	lint.BeginPosition = 2
	lint.MaxLength = 14
	result.CombinedOutput = ""
	if err := lint.Transform(result); err != nil || result.CombinedOutput != "" {
		t.Fatal(err, result.CombinedOutput)
	}
	result.CombinedOutput = "aaa  \r\n\t"
	if err := lint.Transform(result); err != nil || result.CombinedOutput != "a" {
		t.Fatal(err, result.CombinedOutput)
	}
	result.CombinedOutput = mixedString
	if err := lint.Transform(result); err != nil || result.CombinedOutput != "c def 123;@#$<" {
		t.Fatal(err, result.CombinedOutput)
	}
}

func TestNotifyViaEmail_Transform(t *testing.T) {
	notify := NotifyViaEmail{}
	if notify.IsConfigured() {
		t.Fatal("should not be configured")
	}
	// It simply must not panic
	if err := notify.Transform(&feature.Result{}); err != nil {
		t.Fatal(err)
	}

	notify.Mailer = &email.Mailer{
		MailFrom:       "howard@localhost",
		MTAAddressPort: "localhost:25",
	}
	notify.Recipients = []string{"howard@localhost"}
	if !notify.IsConfigured() {
		t.Fatal("should be configured now")
	}

	// It simply must not panic
	if err := notify.Transform(&feature.Result{}); err != nil {
		t.Fatal(err)
	}
}

func TestResetCombinedText_Transform(t *testing.T) {
	combo := ResetCombinedText{}
	result := &feature.Result{}
	if err := combo.Transform(result); err != nil {
		t.Fatal(err)
	}

	result.Error = errors.New("test")
	if err := combo.Transform(result); err != nil {
		t.Fatal(err)
	}
	if result.CombinedOutput != "test" {
		t.Fatal(result.CombinedOutput)
	}
}

func TestSayEmptyOutput_Transform(t *testing.T) {
	empty := SayEmptyOutput{}
	result := &feature.Result{CombinedOutput: "    \t\r\n    "}
	if err := empty.Transform(result); err != nil || result.CombinedOutput != EmptyOutputText {
		t.Fatal(err, result)
	}
	result.CombinedOutput = "test"
	if err := empty.Transform(result); err != nil || result.CombinedOutput != "test" {
		t.Fatal(err, result)
	}
}
