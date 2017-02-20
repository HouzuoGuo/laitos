package feature

import (
	"strconv"
	"testing"
)

func TestFacebook_Execute(t *testing.T) {
	if !TestFacebook.IsConfigured() {
		t.Skip()
	}
	if err := TestFacebook.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := TestFacebook.SelfTest(); err != nil {
		t.Fatal(err)
	}
	// Posting an empty message should result in an error
	if ret := TestFacebook.Execute(Command{TimeoutSec: 30, Content: "  "}); ret.Error == nil ||
		ret.Error != ErrEmptyCommand {
		t.Fatal(ret)
	}
	// Post a good tweet
	message := "test pls ignore"
	if ret := TestFacebook.Execute(Command{TimeoutSec: 30, Content: message}); ret.Error != nil ||
		ret.Output != strconv.Itoa(len(message)) {
		t.Fatal(ret)
	}
}
