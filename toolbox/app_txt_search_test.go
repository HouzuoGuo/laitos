package toolbox

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestTextSearch(t *testing.T) {
	// Prepare feature using incorrect configuration should result in error
	txt := TextSearch{}
	if txt.IsConfigured() {
		t.Fatal("not right")
	}
	txt.FilePaths = map[string]string{
		"a": "file-does-not-exist",
	}
	if err := txt.SelfTest(); err == nil {
		t.Fatal("did not error")
	}
	// Prepare a good file
	tmpTxt, err := os.CreateTemp("", "laitos-test-text-search")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tmpTxt.WriteString(`where The volume button is
and Then How the Hinge works
and How to get help With the New surface
Where is the USB type C port`)
	if err != nil {
		t.Fatal(err)
	}
	err = tmpTxt.Close()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpTxt.Name())
	txt.FilePaths = map[string]string{
		"intro": tmpTxt.Name(),
	}
	if err := txt.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := txt.SelfTest(); err != nil {
		t.Fatal(err)
	}
	// Search for text using bad parameters
	if ret := txt.Execute(context.Background(), Command{Content: ""}); ret.Error == nil {
		t.Fatal("did not error")
	}
	if ret := txt.Execute(context.Background(), Command{Content: "intro"}); ret.Error != ErrBadTextSearchParam {
		t.Fatal(ret.Error)
	}
	if ret := txt.Execute(context.Background(), Command{Content: "shortcut-does-not-exist search this"}); !strings.HasPrefix(ret.Error.Error(), "cannot find") {
		t.Fatal(ret.Error)
	}
	// Search for text using good parameters
	if ret := txt.Execute(context.Background(), Command{Content: "intro then how the "}); ret.Error != nil || ret.Output != "1 and Then How the Hinge works\n" {
		t.Fatal(ret)
	}
	if ret := txt.Execute(context.Background(), Command{Content: "intro where"}); ret.Error != nil || ret.Output != "2 where The volume button is\nWhere is the USB type C port" {
		t.Fatal(ret)
	}
}
