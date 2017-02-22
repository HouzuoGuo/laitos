package bridge

import (
	"errors"
	"github.com/HouzuoGuo/websh/feature"
	"testing"
)

func TestLintCombinedText_Transform(t *testing.T) {
	lint := LintCombinedText{}
	result := &feature.Result{}
	if err := lint.Transform(result); err != nil || result.CombinedOutput != "" {
		t.Fatal(err, result.CombinedOutput)
	}

	mixedString := "abc  def 123 \r\t\n @#$<>\r\t\n 任意的"
	result.CombinedOutput = mixedString
	if err := lint.Transform(result); err != nil || result.CombinedOutput != mixedString {
		t.Fatal(err, result.CombinedOutput)
	}

	result.CombinedOutput = mixedString
	lint.TrimSpaces = true
	lint.CompressToSingleLine = true
	lint.KeepVisible7BitCharOnly = true
	lint.CompressSpaces = true
	lint.BeginPosition = 2
	lint.MaxLength = 14
	if err := lint.Transform(result); err != nil || result.CombinedOutput != "c def 123;@#$<" {
		t.Fatal(err, result.CombinedOutput)
	}
}

func TestNotifyViaEmail_Transform(t *testing.T) {
	email := NotifyViaEmail{}
	if email.IsConfigured() {
		t.Fatal("should not be configured")
	}
	// It simply must not panic
	if err := email.Transform(&feature.Result{}); err != nil {
		t.Fatal(err)
	}

	email.MailFrom = "howard@localhost"
	email.Recipients = []string{"howard@localhost"}
	email.MTAAddressPort = "localhost:25"
	if !email.IsConfigured() {
		t.Fatal("should be configured now")
	}

	// It simply must not panic
	if err := email.Transform(&feature.Result{}); err != nil {
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
