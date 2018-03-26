package httpd

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/httpd/handler"
	"github.com/HouzuoGuo/laitos/inet"
	"io/ioutil"
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

	daemon := Daemon{
		Processor:        nil,
		ServeDirectories: map[string]string{"my/dir": "/tmp/test-laitos-dir", "dir": "/tmp/test-laitos-dir"},
		HandlerCollection: map[string]handler.Handler{
			"/": &handler.HandleHTMLDocument{HTMLFilePath: indexFile},
		},
	}
	// Must not initialise if command processor is not sane
	daemon.Processor = common.GetInsaneCommandProcessor()
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal("did not error due to insane CommandProcessor")
	}
	daemon.Processor = common.GetTestCommandProcessor()
	// Test default settings
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	if daemon.PerIPLimit != 16 || daemon.Port != 80 || daemon.Address != "0.0.0.0" {
		t.Fatalf("%+v", daemon)
	}
	// Prepare settings for test
	daemon.Address = "127.0.0.1"
	daemon.Port = 43250
	daemon.PerIPLimit = 10 // limit must be high enough to tolerate consecutive API endpoint tests
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}

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
	daemon.HandlerCollection["/recurring_cmds"] = &handler.HandleRecurringCommands{
		RecurringCommands: map[string]*common.RecurringCommands{
			"channel1": {
				PreConfiguredCommands: []string{common.TestCommandProcessorPIN + ".s echo -n this is channel1"},
				IntervalSec:           1,
				MaxResults:            4,
			},
			"channel2": {
				PreConfiguredCommands: []string{common.TestCommandProcessorPIN + ".s echo -n this is channel2"},
				IntervalSec:           1,
				MaxResults:            4,
			},
		},
	}
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	for route := range daemon.AllRateLimits {
		fmt.Println("HTTP server has a route", route)
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
