package headless_browser

import (
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"testing"
	"time"
)

func TestBrowserServer_Start(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("Because the built-in PhantomJS executable only works in linux/amd64, your system cannot run this test.")
	}
	phantomJSPath := path.Join(os.Getenv("GOPATH"), "/src/github.com/HouzuoGuo/laitos/addon/phantomjs-2.1.1-linux-x86_64")
	renderOutput, err := ioutil.TempFile("", "laitos-browser-test-render")
	if err != nil {
		t.Fatal(err)
	}
	browser := BrowserServer{
		PhantomJSExecPath: phantomJSPath,
		RenderImagePath:   renderOutput.Name(),
		Port:              41599,
	}
	if err := browser.Start(); err != nil {
		t.Fatal(err)
	}
	defer browser.Stop()
	var result bool
	if err := browser.SendRequest("goto", map[string]interface{}{
		"user_agent":  "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.100 Safari/537.36",
		"view_width":  1280,
		"view_height": 768,
		"page_url":    "https://www.google.com",
	}, &result); err != nil || !result {
		t.Fatal(err, false, browser.GetDebugOutput())
	}
	// Expect page to open within 3 seconds
	time.Sleep(3 * time.Second)
	if err := browser.SendRequest("redraw", nil, &result); err != nil {
		t.Fatal(err, false, browser.GetDebugOutput())
	}
	// Expect page to render within 3 seconds
	time.Sleep(3 * time.Second)
	if stat, err := os.Stat(renderOutput.Name()); err != nil || stat.Size() < 1024 {
		t.Fatal(err, stat.Size(), browser.GetDebugOutput())
	}
	os.Remove(renderOutput.Name())
}
