package filter

import (
	"fmt"
	"testing"

	"github.com/HouzuoGuo/laitos/toolbox"
)

func TestGetTOTP(t *testing.T) {
	// Empty PIN results in no TOTP codes returned
	pin := PINAndShortcuts{}
	if codes := pin.getTOTP(); len(codes) != 0 {
		t.Fatal(codes)
	}
	// Validate code length
	pin.PIN = "abcdefg"
	codes := pin.getTOTP()
	if len(codes) != 9 {
		t.Fatal(codes)
	}
	for code := range codes {
		if len(code) != 12 {
			t.Fatal(codes)
		}
	}
	// Validate code content
	_, current1, _, err := toolbox.GetTwoFACodes("abcdefg")
	if err != nil {
		t.Fatal(err)
	}
	_, current2, _, err := toolbox.GetTwoFACodes("gfedcba")
	if err != nil {
		t.Fatal(err)
	}
	if !codes[current1+current2] {
		t.Fatal("missing code", current1+current2, codes)
	}
}

func TestPINAndShortcuts_Transform(t *testing.T) {
	pin := PINAndShortcuts{}
	if _, err := pin.Transform(toolbox.Command{Content: "abc"}); err == nil {
		t.Fatal("should have been an error")
	}
	// Match shortcut
	pin.Shortcuts = map[string]string{"abc": "123", "def": "456"}
	if out, err := pin.Transform(toolbox.Command{Content: "this is not a matching shortcut"}); err != ErrPINAndShortcutNotFound || out.Content != "this is not a matching shortcut" {
		t.Fatal(out, err)
	}
	// Should stop processing after the first match
	if out, err := pin.Transform(toolbox.Command{Content: "\n\n abc\nrandom line\ndef"}); err != nil || out.Content != "123" {
		t.Fatal(out, err)
	}
	if out, err := pin.Transform(toolbox.Command{Content: "\nrandom line\n\n def \n\n"}); err != nil || out.Content != "456" {
		t.Fatal(out, err)
	}
	// Match PIN
	pin.PIN = "mypin"
	if out, err := pin.Transform(toolbox.Command{Content: "this is not a matching pin"}); err != ErrPINAndShortcutNotFound || out.Content != "this is not a matching pin" {
		t.Fatal(out, err)
	}
	if out, err := pin.Transform(toolbox.Command{Content: "mypineapple"}); err != nil || out.Content != "eapple" {
		t.Fatal(out, err)
	}
	if out, err := pin.Transform(toolbox.Command{Content: "\nrandom line\n mypineapple \nrandom line\n"}); err != nil || out.Content != "eapple" {
		t.Fatal(out, err)
	}
	// Continue to match shortcut when PIN is also configured
	if out, err := pin.Transform(toolbox.Command{Content: "\nrandom line\n\n def \n\n"}); err != nil || out.Content != "456" {
		t.Fatal(out, err)
	}
	// Match TOTP
	_, current1, _, err := toolbox.GetTwoFACodes("mypin")
	if err != nil {
		t.Fatal(err)
	}
	_, current2, _, err := toolbox.GetTwoFACodes("nipym")
	if err != nil {
		t.Fatal(err)
	}
	if out, err := pin.Transform(toolbox.Command{Content: "this is not a matching totp"}); err != ErrPINAndShortcutNotFound || out.Content != "this is not a matching totp" {
		t.Fatal(out, err)
	}
	if out, err := pin.Transform(toolbox.Command{Content: current1 + current2 + "apple"}); err != nil || out.Content != "apple" {
		t.Fatal(out, err)
	}
	if out, err := pin.Transform(toolbox.Command{Content: fmt.Sprintf("\nline1\n %s%sapple\nline\n", current1, current2)}); err != nil || out.Content != "apple" {
		t.Fatal(out, err)
	}
}

func TestTranslateSequences_Transform(t *testing.T) {
	tr := TranslateSequences{}
	if out, err := tr.Transform(toolbox.Command{Content: "abc"}); err != nil || out.Content != "abc" {
		t.Fatal(out)
	}
	tr.Sequences = [][]string{{"abc", "123"}, {"def", "456"}, {"bad tuple"}}
	if out, err := tr.Transform(toolbox.Command{Content: ""}); err != nil || out.Content != "" {
		t.Fatal(out)
	}
	if out, err := tr.Transform(toolbox.Command{Content: " abc def "}); err != nil || out.Content != " 123 456 " {
		t.Fatal(out)
	}
	if out, err := tr.Transform(toolbox.Command{Content: " ghi "}); err != nil || out.Content != " ghi " {
		t.Fatal(out)
	}
}
