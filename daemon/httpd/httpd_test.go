package httpd

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/httpd/handler"
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

	daemon := Daemon{
		Address:          "127.0.0.1",
		Port:             1024 + rand.Intn(65535-1024),
		Processor:        nil,
		ServeDirectories: map[string]string{"my/dir": "/tmp/test-laitos-dir", "dir": "/tmp/test-laitos-dir"},
		BaseRateLimit:    0,
		HandlerCollection: map[string]handler.Handler{
			"/": &handler.HandleHTMLDocument{HTMLFilePath: indexFile},
		},
	}
	// Must not initialise if rate limit is too small
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "BaseRateLimit") {
		t.Fatal(err)
	}
	daemon.BaseRateLimit = 10 // good enough for both sets of test cases
	// Must be able to initialise if command processor is empty (not used)
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Must not initialise if command processor is not sane
	daemon.Processor = common.GetInsaneCommandProcessor()
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal("did not error due to insane CommandProcessor")
	}
	daemon.Processor = common.GetTestCommandProcessor()

	// Set up API handlers
	daemon.Processor = common.GetTestCommandProcessor()
	daemon.HandlerCollection["/info"] = &handler.HandleSystemInfo{FeaturesToCheck: daemon.Processor.Features}
	daemon.HandlerCollection["/cmd_form"] = &handler.HandleCommandForm{}
	daemon.HandlerCollection["/gitlab"] = &handler.HandleGitlabBrowser{PrivateToken: "token-does-not-matter-in-this-test"}
	daemon.HandlerCollection["/html"] = &handler.HandleHTMLDocument{HTMLFilePath: indexFile}
	daemon.HandlerCollection["/mail_me"] = &handler.HandleMailMe{
		Recipients: []string{"howard@localhost"},
		MailClient: inet.MailClient{
			MailFrom: "howard@localhost",
			MTAHost:  "localhost",
			MTAPort:  25,
		},
	}
	daemon.HandlerCollection["/microsoft_bot"] = &handler.HandleMicrosoftBot{
		ClientAppID:     "dummy ID",
		ClientAppSecret: "dummy secret",
	}
	daemon.HandlerCollection["/proxy"] = &handler.HandleWebProxy{OwnEndpoint: "/proxy"}
	daemon.HandlerCollection["/sms"] = &handler.HandleTwilioSMSHook{}
	daemon.HandlerCollection["/call_greeting"] = &handler.HandleTwilioCallHook{CallGreeting: "Hi there", CallbackEndpoint: "/test"}
	daemon.HandlerCollection["/call_command"] = &handler.HandleTwilioCallCallback{MyEndpoint: "/endpoint-does-not-matter-in-this-test"}
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
		if err := daemon.StartAndBlockNoTLS(0); err != nil {
			t.Fatal(err)
		}
		stoppedNormally = true
	}()
	time.Sleep(2 * time.Second)
	TestHTTPD(&daemon, t)
	TestAPIHandlers(&daemon, t)

	// Daemon must stop in a second
	daemon.StopNoTLS()
	time.Sleep(1 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	daemon.StopNoTLS()
	daemon.StopNoTLS()
}
