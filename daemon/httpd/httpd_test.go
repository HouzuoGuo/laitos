package httpd

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/httpd/api"
	"github.com/HouzuoGuo/laitos/inet"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHTTPD_StartAndBlock(t *testing.T) {
	// Create a temporary file for index
	indexFile := "/tmp/test-laitos-index.html"
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

	rand.Seed(time.Now().UnixNano())

	daemon := HTTPD{
		Address:          "127.0.0.1",
		Port:             1024 + rand.Intn(65535-1024),
		Processor:        &common.CommandProcessor{},
		ServeDirectories: map[string]string{"my/dir": "/tmp/test-laitos-dir", "dir": "/tmp/test-laitos-dir"},
		BaseRateLimit:    0,
		SpecialHandlers: map[string]api.HandlerFactory{
			"/": &api.HandleHTMLDocument{HTMLFilePath: indexFile},
		},
	}
	// Must not initialise if command processor is insane
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal("did not error due to insane CommandProcessor")
	}
	daemon.Processor = common.GetTestCommandProcessor()
	// Must not initialise if rate limit is too small
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "BaseRateLimit") {
		t.Fatal(err)
	}
	daemon.BaseRateLimit = 10 // good enough for both sets of test cases

	// Set up API handlers
	daemon.Processor = common.GetTestCommandProcessor()
	daemon.SpecialHandlers["/info"] = &api.HandleSystemInfo{FeaturesToCheck: daemon.Processor.Features}
	daemon.SpecialHandlers["/cmd_form"] = &api.HandleCommandForm{}
	daemon.SpecialHandlers["/gitlab"] = &api.HandleGitlabBrowser{PrivateToken: "token-does-not-matter-in-this-test"}
	daemon.SpecialHandlers["/html"] = &api.HandleHTMLDocument{HTMLFilePath: indexFile}
	daemon.SpecialHandlers["/mail_me"] = &api.HandleMailMe{
		Recipients: []string{"howard@localhost"},
		Mailer: inet.Mailer{
			MailFrom: "howard@localhost",
			MTAHost:  "localhost",
			MTAPort:  25,
		},
	}
	daemon.SpecialHandlers["/microsoft_bot"] = &api.HandleMicrosoftBot{
		ClientAppID:     "dummy ID",
		ClientAppSecret: "dummy secret",
	}
	daemon.SpecialHandlers["/proxy"] = &api.HandleWebProxy{MyEndpoint: "/proxy"}
	daemon.SpecialHandlers["/sms"] = &api.HandleTwilioSMSHook{}
	daemon.SpecialHandlers["/call_greeting"] = &api.HandleTwilioCallHook{CallGreeting: "Hi there", CallbackEndpoint: "/test"}
	daemon.SpecialHandlers["/call_command"] = &api.HandleTwilioCallCallback{MyEndpoint: "/endpoint-does-not-matter-in-this-test"}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	for route := range daemon.AllRateLimits {
		fmt.Println("install route", route)
	}
	// Start server and run tests
	// HTTP daemon is expected to start in two seconds
	var stoppedNormally bool
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stoppedNormally = true
	}()
	time.Sleep(2 * time.Second)
	TestHTTPD(&daemon, t)
	TestAPIHandlers(&daemon, t)

	// Daemon must stop in a second
	daemon.Stop()
	time.Sleep(1 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	daemon.Stop()
	daemon.Stop()
}
