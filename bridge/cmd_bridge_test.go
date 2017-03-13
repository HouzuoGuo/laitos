package bridge

import (
	"github.com/HouzuoGuo/laitos/feature"
	"testing"
)

func TestPINAndShortcuts_Transform(t *testing.T) {
	pin := PINAndShortcuts{}
	if _, err := pin.Transform(feature.Command{Content: "abc"}); err == nil {
		t.Fatal("should have been an error")
	}

	pin = PINAndShortcuts{PIN: "mypin"}
	if out, err := pin.Transform(feature.Command{Content: "abc"}); err != ErrPINAndShortcutNotFound || out.Content != "abc" {
		t.Fatal(out, err)
	}
	if out, err := pin.Transform(feature.Command{Content: "mypineapple"}); err != nil || out.Content != "eapple" {
		t.Fatal(out, err)
	}
	if out, err := pin.Transform(feature.Command{Content: "\nline\n mypineapple \nline\n"}); err != nil || out.Content != "eapple" {
		t.Fatal(out, err)
	}
	pin.Shortcuts = map[string]string{"abc": "123", "def": "456"}
	if out, err := pin.Transform(feature.Command{Content: "nothing_to_see"}); err != ErrPINAndShortcutNotFound || out.Content != "nothing_to_see" {
		t.Fatal(out, err)
	}
	if out, err := pin.Transform(feature.Command{Content: "\n\n mypineapple \n\n"}); err != nil || out.Content != "eapple" {
		t.Fatal(out, err)
	}
	if out, err := pin.Transform(feature.Command{Content: "\n\n abc\nline"}); err != nil || out.Content != "123" {
		t.Fatal(out, err)
	}
	if out, err := pin.Transform(feature.Command{Content: "\nline\n\n def \n\n"}); err != nil || out.Content != "456" {
		t.Fatal(out, err)
	}
	if out, err := pin.Transform(feature.Command{Content: "ghi"}); err != ErrPINAndShortcutNotFound || out.Content != "ghi" {
		t.Fatal(out, err)
	}
}

func TestTranslateSequences_Transform(t *testing.T) {
	tr := TranslateSequences{}
	if out, err := tr.Transform(feature.Command{Content: "abc"}); err != nil || out.Content != "abc" {
		t.Fatal(out)
	}
	tr.Sequences = [][]string{{"abc", "123"}, {"def", "456"}, {"bad tuple"}}
	if out, err := tr.Transform(feature.Command{Content: ""}); err != nil || out.Content != "" {
		t.Fatal(out)
	}
	if out, err := tr.Transform(feature.Command{Content: " abc def "}); err != nil || out.Content != " 123 456 " {
		t.Fatal(out)
	}
	if out, err := tr.Transform(feature.Command{Content: " ghi "}); err != nil || out.Content != " ghi " {
		t.Fatal(out)
	}
}
