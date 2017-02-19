package bridge

import "testing"

func TestStringLint_Transform(t *testing.T) {
	lint := StringLint{}
	if out, err := lint.Transform(""); err != nil || out != "" {
		t.Fatal(err, out)
	}
	mixedString := "abc  def 123 \r\t\n @#$<>\r\t\n 任意的"
	if out, err := lint.Transform(mixedString); err != nil || out != mixedString {
		t.Fatal(err, out)
	}
	lint.TrimSpaces = true
	lint.CompressToSingleLine = true
	lint.KeepVisible7BitCharOnly = true
	lint.CompressSpaces = true
	lint.BeginPosition = 2
	lint.MaxLength = 14
	if out, err := lint.Transform(mixedString); err != nil || out != "c def 123;@#$<" {
		t.Fatal(err, out)
	}
}
