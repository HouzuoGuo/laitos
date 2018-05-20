package toolbox

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/browserp"
	"github.com/HouzuoGuo/laitos/misc"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBrowser_Execute(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("Because the built-in PhantomJS executable only works in linux/amd64, your system cannot run this test.")
	}
	// Preparation copies PhantomJS executable into a utilities directory and adds it to program $PATH.
	misc.PrepareUtilities(misc.Logger{})
	// CircleCI container does not have the dependencies for running PhantomJS
	misc.SkipTestIfCI(t)
	bro := Browser{}
	if bro.IsConfigured() {
		t.Fatal("should not be configured")
	}
	bro.Renderers = &browserp.Instances{
		MaxLifetimeSec: 60,
		BasePortNumber: 60122,
	}
	if !bro.IsConfigured() {
		t.Fatal("should be configured")
	}
	if err := bro.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := bro.SelfTest(); err != nil {
		t.Fatal(err)
	}
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "haha hoho"}); ret.Error != ErrBadBrowserParam {
		t.Fatal(ret.Error, ret.Output)
	}
	delay := func() {
		time.Sleep(2 * time.Second)
	}
	// Browse github home page
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "g https://github.com"}); ret.Error != nil || !strings.Contains(ret.Output, "github") {
		t.Fatal(ret.Error, ret.Output)
	}
	delay()
	// Go back and forward
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "b"}); ret.Error != nil || !strings.Contains(ret.Output, "github") {
		t.Fatal(ret.Error, ret.Output)
	} else {
		fmt.Println(ret.Output)
	}
	delay()
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "f"}); ret.Error != nil || !strings.Contains(ret.Output, "github") {
		t.Fatal(ret.Error, ret.Output)
	} else {
		fmt.Println(ret.Output)
	}
	delay()
	// Navigate to elements
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "n"}); ret.Error != nil || len(ret.Output) < 20 {
		t.Fatal(ret.Error, ret.Output)
	} else {
		fmt.Println(ret.Output)
	}
	delay()
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "p"}); ret.Error != nil || len(ret.Output) < 20 {
		t.Fatal(ret.Error, ret.Output)
	} else {
		fmt.Println(ret.Output)
	}
	delay()
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "nn 10"}); ret.Error != nil || len(ret.Output) < 200 {
		t.Fatal(ret.Error, ret.Output)
	} else {
		fmt.Println(ret.Output)
	}
	delay()
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "0"}); ret.Error != nil || len(ret.Output) < 20 {
		t.Fatal(ret.Error, ret.Output)
	} else {
		fmt.Println(ret.Output)
	}
	delay()
	// Reload and get page info
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "r"}); ret.Error != nil || !strings.Contains(ret.Output, "github") {
		t.Fatal(ret.Error, ret.Output)
	} else {
		fmt.Println(ret.Output)
	}
	delay()
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "i"}); ret.Error != nil || !strings.Contains(ret.Output, "github") {
		t.Fatal(ret.Error, ret.Output)
	} else {
		fmt.Println(ret.Output)
	}
	delay()
	// Pointer, enter value, and keys
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "ptr move left"}); ret.Error != nil || !strings.Contains(ret.Output, "github") {
		t.Fatal(ret.Error, ret.Output)
	} else {
		fmt.Println(ret.Output)
	}
	delay()
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "val new value hahaha"}); ret.Error != nil || !strings.Contains(ret.Output, "github") {
		t.Fatal(ret.Error, ret.Output)
	} else {
		fmt.Println(ret.Output)
	}
	delay()
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "enter"}); ret.Error != nil || !strings.Contains(ret.Output, "github") {
		t.Fatal(ret.Error, ret.Output)
	} else {
		fmt.Println(ret.Output)
	}
	delay()
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "backsp"}); ret.Error != nil || !strings.Contains(ret.Output, "github") {
		t.Fatal(ret.Error, ret.Output)
	} else {
		fmt.Println(ret.Output)
	}
	delay()
	// Kill browser finally
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "k"}); ret.Error != nil || !strings.Contains(ret.Output, "killed") {
		t.Fatal(ret.Error, ret.Output)
	} else {
		fmt.Println(ret.Output)
	}
	delay()
	// Make sure a new browser may start again
	if ret := bro.Execute(Command{TimeoutSec: 10, Content: "g https://github.com"}); ret.Error != nil || !strings.Contains(ret.Output, "github") {
		t.Fatal(ret.Error, ret.Output)
	}
}
