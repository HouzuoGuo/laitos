package httpd

import (
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/frontend/httpd/api"
	"github.com/HouzuoGuo/laitos/global"
	"github.com/HouzuoGuo/laitos/httpclient"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// TODO: upgrade to go 1.8 and implement graceful httpd shutdown.
func TestHTTPD_StartAndBlock(t *testing.T) {
	// Create a temporary file for index
	indexFile := "/tmp/test-laitos-index.html"
	defer os.Remove(indexFile)
	if err := ioutil.WriteFile(indexFile, []byte("this is index #LAITOS_CLIENTADDR #LAITOS_3339TIME"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a temporary directory of file
	htmlDir := "/tmp/test-laitos-dir"
	if err := os.MkdirAll(htmlDir, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(htmlDir)
	if err := ioutil.WriteFile(htmlDir+"/a.html", []byte("a html"), 0644); err != nil {
		t.Fatal(err)
	}

	daemon := HTTPD{
		ListenAddress:    "127.0.0.1",
		ListenPort:       13589, // hard coded port is a random choice
		Processor:        &common.CommandProcessor{},
		ServeDirectories: map[string]string{"my/dir": "/tmp/test-laitos-dir"},
		BaseRateLimit:    1,
		SpecialHandlers: map[string]api.HandlerFactory{
			"/":     &api.HandleHTMLDocument{HTMLFilePath: indexFile},
			"/info": &api.HandleSystemInfo{},
		},
	}
	// Must not initialise if command processor is insane
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal("did not error due to insane CommandProcessor")
	}
	daemon.Processor = common.GetTestCommandProcessor()
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// HTTP daemon is expected to start in two seconds
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)

	addr := "http://127.0.0.1:13589"

	// Index handle
	resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+"/")
	expected := "this is index 127.0.0.1 " + time.Now().Format(time.RFC3339)
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body), expected, resp)
	}
	// Directory handle
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != `<pre>
<a href="a.html">a.html</a>
</pre>
` {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir/a.html")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != "a html" {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Non-existent paths
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir/doesnotexist.html")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/doesnotexist")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Test hitting rate limits
	time.Sleep(RateLimitIntervalSec * time.Second)
	success := 0
	for i := 0; i < 100; i++ {
		resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/")
		expected := "this is index 127.0.0.1 " + time.Now().Format(time.RFC3339)
		if err == nil && resp.StatusCode == http.StatusOK && string(resp.Body) == expected {
			success++
		}
	}
	if success > 15 || success < 5 {
		t.Fatal(success)
	}
	// Wait till rate limits reset
	time.Sleep(RateLimitIntervalSec * time.Second)
	// Trigger a special handle to test routing
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/info")
	if err != nil || resp.StatusCode != http.StatusOK || strings.Index(string(resp.Body), "All OK") == -1 {
		t.Fatal(err, string(resp.Body), resp)
	}

	// Trigger emergency stop, HTTP endpoints shall respond with HTTP 200 and emergency stop message.
	global.TriggerEmergencyLockDown()
	for _, endpoint := range []string{"/", "/info"} {
		resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+endpoint)
		if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != global.ErrEmergencyLockDown.Error() {
			t.Fatal(err, string(resp.Body), err != nil, resp.StatusCode, string(resp.Body) != global.ErrEmergencyLockDown.Error())
		}
	}
	global.EmergencyLockDown = false
}
