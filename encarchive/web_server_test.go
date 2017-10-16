package encarchive

import (
	"github.com/HouzuoGuo/laitos/inet"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
)

func TestGetSysInfoText(t *testing.T) {
	txt := GetSysInfoText()
	if !strings.Contains(txt, "Clock") || len(txt) < 30 {
		t.Fatal(txt)
	}
}

func TestWebServer(t *testing.T) {
	emptyTmpFile, err := ioutil.TempFile("", "laitos-encarchive-TestWebServer")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(emptyTmpFile.Name())
	ws := WebServer{
		Port:            54396,
		URL:             "/test-url",
		ArchiveFilePath: "/",
	}
	var shutdown bool
	go func() {
		ws.Start()
		shutdown = true
	}()
	// Expect server to start within a second
	time.Sleep(1 * time.Second)
	resp, err := inet.DoHTTP(inet.HTTPRequest{}, "http://localhost:54396")
	// Access any URL but the correct URL will return empty body
	if err != nil || string(resp.Body) != "" {
		t.Fatal(err, string(resp.Body))
	}
	// Access the correct URL for password unlock page
	resp, err = inet.DoHTTP(inet.HTTPRequest{}, "http://localhost:54396/test-url")
	if err != nil || !strings.Contains(string(resp.Body), "Clock") || !strings.Contains(string(resp.Body), "Enter password") {
		t.Fatal(string(resp.Body))
	}
	// Server should shut down within a second
	err = ws.Shutdown()
	time.Sleep(1 * time.Second)
	if err != nil || !shutdown {
		t.Fatal(err, shutdown)
	}
}
