package browser

import (
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"testing"
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
	instance := &Renderer{
		PhantomJSExecPath:  phantomJSPath,
		RenderImagePath:    renderOutput.Name() + ".png",
		Port:               41599,
		AutoKillTimeoutSec: 30,
	}
	if err := instance.Start(); err != nil {
		t.Fatal(err)
	}
	defer instance.Kill()
	var result bool
	if err := instance.SendRequest("goto", map[string]interface{}{
		"user_agent":  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36",
		"view_width":  1280,
		"view_height": 768,
		"page_url":    "https://www.google.com",
	}, &result); err != nil || !result {
		t.Fatal(err, instance.GetDebugOutput(1000))
	}
	if err := instance.RenderPage(); err != nil {
		t.Fatal(err)
	}
	if stat, err := os.Stat(instance.RenderImagePath); err != nil || stat.Size() < 1024 {
		t.Fatal(err, stat.Size(), instance.GetDebugOutput(1000))
	}
	os.Remove(instance.RenderImagePath)
	// Expect some output to be already present in output buffer
	t.Log(instance.GetDebugOutput(1000))
	// Last line should be "POST /redraw - {}: true\n"
	if out := instance.GetDebugOutput(5); out != "true\n" {
		t.Fatalf(out)
	}
	// Repeatedly stopping instance should have no negative consequence
	instance.Kill()
	instance.Kill()
	instance.Kill()
}
