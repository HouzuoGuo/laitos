package phantomjs

import (
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/platform"
)

func TestInteractiveBrowser(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("Because the built-in PhantomJS executable only works in linux/amd64, your system cannot run this test.")
	}
	// Preparation copies PhantomJS executable into a utilities directory and adds it to program $PATH.
	platform.CopyNonEssentialUtilities(lalog.Logger{})
	// CircleCI container does not have the dependencies for running PhantomJS
	platform.SkipTestIfCI(t)
	renderOutput, err := ioutil.TempFile("", "laitos-TestInteractiveBrowser-phantomjs")
	if err != nil {
		t.Fatal(err)
	}
	instance := &Instance{
		RenderImagePath:    renderOutput.Name() + ".jpg",
		PhantomJSExecPath:  "phantomjs", // PrepareUtilities makes it available
		Port:               22987,
		AutoKillTimeoutSec: 300,
	}
	if err := instance.Start(); err != nil {
		t.Fatal(err)
	}
	defer instance.Kill()
	// Browse distrowatch home page
	if err := instance.GoTo(GoodUserAgent, "https://distrowatch.com/", 1024, 1024); err != nil {
		t.Fatal(err, instance.GetDebugOutput())
	}
	// Expect page to be ready soon
	time.Sleep(15 * time.Second)
	if err := instance.RenderPage(); err != nil {
		t.Fatal(err)
	}
	if stat, err := os.Stat(instance.RenderImagePath); err != nil || stat.Size() < 4096 {
		t.Fatal(err, stat.Size(), instance.GetDebugOutput())
	}
	os.Remove(instance.RenderImagePath)
	// Expect some output to be already present in output buffer
	t.Log(instance.GetDebugOutput())
	// The image render action should have written a line of log that looks like "POST /redraw - {}: true\n"
	if out := instance.GetDebugOutput(); !strings.Contains(out, "/redraw") {
		t.Fatalf(out)
	}
	// Try several other browser actions
	if err := instance.GoBack(); err != nil {
		t.Fatal(err)
	}
	if err := instance.GoForward(); err != nil {
		t.Fatal(err)
	}
	if err := instance.Reload(); err != nil {
		t.Fatal(err)
	}
	if err := instance.Pointer(PointerTypeClick, PointerButtonRight, 100, 100); err != nil {
		t.Fatal(err)
	}
	if err := instance.SendKey("test string", 0); err != nil {
		t.Fatal(err)
	}
	if err := instance.SendKey("", 1234); err != nil {
		t.Fatal(err)
	}
	// Repeatedly stopping instance should have no negative consequence
	instance.Kill()
	instance.Kill()
	instance.Kill()
}

func TestLineOrientedBrowser(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("Because the built-in PhantomJS executable only works in linux/amd64, your system cannot run this test.")
	}
	// Preparation copies PhantomJS executable into a utilities directory and adds it to program $PATH.
	platform.CopyNonEssentialUtilities(lalog.Logger{})
	// CircleCI container does not have the dependencies for running PhantomJS
	platform.SkipTestIfCI(t)
	renderOutput, err := ioutil.TempFile("", "laitos-TestLineOrientedBrowser-phantomjs")
	if err != nil {
		t.Fatal(err)
	}
	instance := &Instance{
		RenderImagePath:    renderOutput.Name() + ".jpg",
		PhantomJSExecPath:  "phantomjs", // PrepareUtilities makes it available
		Port:               48111,
		AutoKillTimeoutSec: 300,
	}
	if err := instance.Start(); err != nil {
		t.Fatal(err)
	}
	defer instance.Kill()
	// Browse distrowatch home page
	if err := instance.GoTo(GoodUserAgent, "https://distrowatch.com/", 1024, 1024); err != nil {
		t.Fatal(err, instance.GetDebugOutput())
	}
	// Expect page to be ready in a few seconds
	time.Sleep(30 * time.Second)
	delay := func() {
		time.Sleep(3 * time.Second)
	}
	// Navigate to first element
	firstElements, err := instance.LONextElement()
	if err != nil || len(firstElements) != 3 {
		t.Fatal(err, firstElements)
	}
	// [0] should be empty because there is no previous element
	if firstElements[0].TagName != "" || firstElements[1].TagName == "" || firstElements[2].TagName == "" {
		t.Fatal(err, firstElements)
	}
	delay()
	// Navigate to second element
	secondElements, err := instance.LONextElement()
	if err != nil || len(secondElements) != 3 {
		t.Fatal(err, secondElements)
	}
	delay()
	// [1] should match the previous element's next element
	if secondElements[1].TagName != firstElements[2].TagName || secondElements[1].TagName == "" || secondElements[2].TagName == "" {
		t.Fatal(err, secondElements[1].TagName, firstElements[2].TagName, secondElements)
	}
	delay()
	// Navigate all the way to the bottom
	bottom, err := instance.LONextNElements(10000)
	if err != nil || len(bottom) < 30 {
		t.Fatal(err, bottom)
	}
	delay()
	// After having reached the bottom, calling next should stay at bottom.
	var bottomElements []ElementInfo
	for i := 0; i < 3; i++ {
		bottomElements, err = instance.LONextElement()
		if err != nil {
			t.Fatal(err)
		}
		delay()
	}
	lastElements, err := instance.LONextElement()
	if err != nil || lastElements[1].TagName != bottomElements[1].TagName {
		t.Fatalf("%+v\n%+v\n%+v\n", err, lastElements, bottomElements)
	}
	delay()
	// Go back to the start
	if err := instance.LOResetNavigation(); err != nil {
		t.Fatal(err)
	}
	delay()
	// Try pointer and value actions
	if err := instance.LOPointer(PointerTypeMove, PointerButtonLeft); err != nil {
		t.Fatal(err)
	}
	delay()
	if err := instance.LOSetValue("test value"); err != nil {
		t.Fatal(err)
	}
}
