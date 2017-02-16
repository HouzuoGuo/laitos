package bridge

import (
	"github.com/HouzuoGuo/websh/feature"
	"testing"
)

func TestCommandShortcuts_Transform(t *testing.T) {
	short := CommandShortcuts{}
	if out := short.Transform(feature.Command{Content: "abc"}); out.Content != "abc" {
		t.Fatal(out)
	}
	short.Shortcuts = map[string]string{"abc": "123", "def": "456"}
	if out := short.Transform(feature.Command{Content: ""}); out.Content != "" {
		t.Fatal(out)
	}
	if out := short.Transform(feature.Command{Content: "abc"}); out.Content != "123" {
		t.Fatal(out)
	}
	if out := short.Transform(feature.Command{Content: " def "}); out.Content != "456" {
		t.Fatal(out)
	}
	if out := short.Transform(feature.Command{Content: "ghi"}); out.Content != "ghi" {
		t.Fatal(out)
	}
}

func TestCommandTranslator_Transform(t *testing.T) {
	tr := CommandTranslator{}
	if out := tr.Transform(feature.Command{Content: "abc"}); out.Content != "abc" {
		t.Fatal(out)
	}
	tr.Sequences = [][]string{{"abc", "123"}, {"def", "456"}}
	if out := tr.Transform(feature.Command{Content: ""}); out.Content != "" {
		t.Fatal(out)
	}
	if out := tr.Transform(feature.Command{Content: " abc def "}); out.Content != " 123 456 " {
		t.Fatal(out)
	}
	if out := tr.Transform(feature.Command{Content: " ghi "}); out.Content != " ghi " {
		t.Fatal(out)
	}
}

func TestStringLint_Transform(t *testing.T) {
	lint := StringLint{}
	if out := lint.Transform(""); out != "" {
		t.Fatal(out)
	}
	mixedString := "abc  def 123 \r\t\n @#$<>\r\t\n 任意的"
	if out := lint.Transform(mixedString); out != mixedString {
		t.Fatal(out)
	}
	lint.TrimSpaces = true
	lint.CompressToSingleLine = true
	lint.KeepVisible7BitCharOnly = true
	lint.CompressSpaces = true
	lint.BeginPosition = 2
	lint.MaxLength = 14
	if out := lint.Transform(mixedString); out != "c def 123 ;@#" {
		t.Fatal(out)
	}
}
