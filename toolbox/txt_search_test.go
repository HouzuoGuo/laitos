package toolbox

import (
	"io/ioutil"
	"os"
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
	tmpTxt, err := ioutil.TempFile("", "laitos-test-text-search")
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
	if result := txt.Execute(Command{Content: ""}); result.Error == nil {
		t.Fatal("did not error")
	}
	if result := txt.Execute(Command{Content: "intro"}); result.Error == nil {
		t.Fatal("did not error")
	}
	if result := txt.Execute(Command{Content: "shortcut-does-not-exist search this"}); result.Error == nil {
		t.Fatal("did not error")
	}
	// Search for text using good parameters
	if result := txt.Execute(Command{Content: "intro then how the "}); result.Error != nil || result.Output != "1 and Then How the Hinge works\n" {
		t.Fatal(result)
	}
	if result := txt.Execute(Command{Content: "intro where"}); result.Error != nil || result.Output != "2 where The volume button is\nWhere is the USB type C port" {
		t.Fatal(result)
	}
}
