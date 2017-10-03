package browser

import (
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"time"
)

var phantomJSPath = path.Join(os.Getenv("GOPATH"), "/src/github.com/HouzuoGuo/laitos/extra/phantomjs-2.1.1-linux-x86_64")

func TestInteractiveBrowser(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("Because the built-in PhantomJS executable only works in linux/amd64, your system cannot run this test.")
	}
	renderOutput, err := ioutil.TempFile("", "laitos-TestInteractiveBrowser")
	if err != nil {
		t.Fatal(err)
	}
	instance := &Instance{
		PhantomJSExecPath:  phantomJSPath,
		RenderImagePath:    renderOutput.Name() + ".png",
		Port:               41599,
		AutoKillTimeoutSec: 30,
	}
	if err := instance.Start(); err != nil {
		t.Fatal(err)
	}
	defer instance.Kill()
	// Browse microsoft home page
	if err := instance.GoTo(GoodUserAgent, "https://www.microsoft.com", 1024, 1024); err != nil {
		t.Fatal(err, instance.GetDebugOutput(1000))
	}
	// Expect page to be ready in three seconds
	time.Sleep(3 * time.Second)
	if err := instance.RenderPage(); err != nil {
		t.Fatal(err)
	}
	if stat, err := os.Stat(instance.RenderImagePath); err != nil || stat.Size() < 4096 {
		t.Fatal(err, stat.Size(), instance.GetDebugOutput(1000))
	}
	os.Remove(instance.RenderImagePath)
	// Expect some output to be already present in output buffer
	t.Log(instance.GetDebugOutput(1000))
	// The image render action should have written a line of log that looks like "POST /redraw - {}: true\n"
	if out := instance.GetDebugOutput(500); !strings.Contains(out, "/redraw - {}: true") {
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
	renderOutput, err := ioutil.TempFile("", "laitos-TestLineOrientedBrowser")
	if err != nil {
		t.Fatal(err)
	}
	instance := &Instance{
		PhantomJSExecPath:  phantomJSPath,
		RenderImagePath:    renderOutput.Name() + ".png",
		Port:               51600,
		AutoKillTimeoutSec: 30,
	}
	if err := instance.Start(); err != nil {
		t.Fatal(err)
	}
	defer instance.Kill()
	// Browse github home page
	if err := instance.GoTo(GoodUserAgent, "https://github.com", 1024, 1024); err != nil {
		t.Fatal(err, instance.GetDebugOutput(1000))
	}
	// Expect page to be ready in three seconds
	time.Sleep(3 * time.Second)
	delay := func() {
		time.Sleep(2 * time.Second)
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
	elements, err := instance.LONextNElements(10000)
	if err != nil || len(elements) < 30 {
		t.Fatal(err, elements)
	}
	delay()
	// After having reached the bottom, calling next should continue to stay at the bottom.
	lastElements, err := instance.LONextElement()
	if err != nil || lastElements[1].TagName != elements[len(elements)-1].TagName {
		t.Fatal(err, lastElements)
	}
	delay()
	// Go back to the start
	if err := instance.LOResetNavigation(); err != nil {
		t.Fatal(err)
	}
	delay()
	revisitFirstElements, err := instance.LONextElement()
	if err != nil || len(revisitFirstElements) != 3 ||
		revisitFirstElements[0].TagName != "" ||
		revisitFirstElements[1].TagName != firstElements[1].TagName ||
		revisitFirstElements[2].TagName != firstElements[2].TagName {
		t.Fatal(err, revisitFirstElements, firstElements)
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
	delay()
	// Re-visit the second element
	revisitSecondElements, err := instance.LONextElement()
	if err != nil || len(revisitSecondElements) != 3 ||
		revisitSecondElements[0].TagName != secondElements[0].TagName ||
		revisitSecondElements[1].TagName != secondElements[1].TagName ||
		revisitSecondElements[2].TagName != secondElements[2].TagName {
		t.Fatal(err, revisitSecondElements, secondElements)
	}
}
