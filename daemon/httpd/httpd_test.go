package httpd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/httpd/handler"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/stretchr/testify/require"
)

func TestHTTPD_WithURLPrefix(t *testing.T) {
	PrepareForTestHTTPD(t)
	// Prepare directory listing route, text file rendering route, and a handler function route.
	daemon := Daemon{
		Address:          "localhost",
		Port:             34859,
		Processor:        toolbox.GetTestCommandProcessor(),
		ServeDirectories: map[string]string{"my/dir": "/tmp/test-laitos-dir", "dir": "/tmp/test-laitos-dir"},
		HandlerCollection: map[string]handler.Handler{
			"/": &handler.HandleHTMLDocument{HTMLFilePath: "/tmp/test-laitos-index.html"},
		},
	}
	daemon.HandlerCollection["/info"] = &handler.HandleSystemInfo{FeaturesToCheck: daemon.Processor.Features}
	if err := daemon.Initialise("/test-prefix/abc", ""); err != nil {
		t.Fatalf("%+v", err)
	}
	// Expect server to startup within two seconds
	go func() {
		if err := daemon.StartAndBlockNoTLS(0); err != nil {
			t.Error(err)
			return
		}
	}()
	if !misc.ProbePort(30*time.Second, daemon.Address, daemon.Port) {
		t.Fatal("server did not start in time")
	}
	// Test directory listing route
	serverURLPrefix := fmt.Sprintf("http://localhost:%d/test-prefix/abc", daemon.Port)
	resp, err := inet.DoHTTP(context.Background(), inet.HTTPRequest{}, serverURLPrefix+"/my/dir")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(resp.Body), "a.html")
	// Test file route
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, serverURLPrefix+"/")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "this is index") {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Test info handler function route
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, serverURLPrefix+"/info")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "Stack traces:") {
		t.Fatal(err, string(resp.Body), resp)
	}

	daemon.StopNoTLS()
}

func TestHTTPD_StartAndBlock(t *testing.T) {
	PrepareForTestHTTPD(t)
	daemon := Daemon{
		Processor:        nil,
		ServeDirectories: map[string]string{"my/dir": "/tmp/test-laitos-dir", "dir": "/tmp/test-laitos-dir"},
		HandlerCollection: map[string]handler.Handler{
			"/": &handler.HandleHTMLDocument{HTMLFilePath: "/tmp/test-laitos-index.html"},
		},
	}
	// Must not initialise if command processor is not sane
	daemon.Processor = toolbox.GetInsaneCommandProcessor()
	if err := daemon.Initialise("", ""); err == nil || !strings.Contains(err.Error(), toolbox.ErrBadProcessorConfig) {
		t.Fatal("did not error due to insane CommandProcessor")
	}
	daemon.Processor = toolbox.GetTestCommandProcessor()
	// Test default settings
	if err := daemon.Initialise("", ""); err != nil {
		t.Fatal(err)
	}
	if daemon.PerIPLimit != 12 || daemon.Port != 80 || daemon.Address != "0.0.0.0" {
		t.Fatalf("%+v", daemon)
	}
	// Prepare settings for test
	daemon.Address = "127.0.0.1"
	daemon.Port = 43250
	daemon.PerIPLimit = 10 // limit must be high enough to tolerate consecutive API endpoint tests
	if err := daemon.Initialise("", ""); err != nil {
		t.Fatal(err)
	}

	// Set up API handlers
	daemon.Processor = toolbox.GetTestCommandProcessor()
	daemon.HandlerCollection["/info"] = &handler.HandleSystemInfo{FeaturesToCheck: daemon.Processor.Features}
	daemon.HandlerCollection["/cmd_form"] = &handler.HandleCommandForm{}
	daemon.HandlerCollection["/upload"] = &handler.HandleFileUpload{}
	daemon.HandlerCollection["/gitlab"] = &handler.HandleGitlabBrowser{PrivateToken: "token-does-not-matter-in-this-test"}
	daemon.HandlerCollection["/html"] = &handler.HandleHTMLDocument{HTMLFilePath: "/tmp/test-laitos-index.html"}
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
	daemon.HandlerCollection["/ttn"] = &handler.HandleLoraWANWebhook{}
	daemon.HandlerCollection["/sms"] = &handler.HandleTwilioSMSHook{}
	daemon.HandlerCollection["/call_greeting"] = &handler.HandleTwilioCallHook{CallGreeting: "Hi there", CallbackEndpoint: "/test"}
	daemon.HandlerCollection["/call_command"] = &handler.HandleTwilioCallCallback{MyEndpoint: "/endpoint-does-not-matter-in-this-test"}
	daemon.HandlerCollection["/recurring_cmds"] = &handler.HandleRecurringCommands{
		RecurringCommands: map[string]*common.RecurringCommands{
			"channel1": {
				PreConfiguredCommands: []string{toolbox.TestCommandProcessorPIN + ".s echo -n this is channel1"},
				IntervalSec:           1,
				MaxResults:            4,
			},
			"channel2": {
				PreConfiguredCommands: []string{toolbox.TestCommandProcessorPIN + ".s echo -n this is channel2"},
				IntervalSec:           1,
				MaxResults:            4,
			},
		},
	}
	daemon.HandlerCollection["/cmd"] = &handler.HandleAppCommand{}
	daemon.HandlerCollection["/reports"] = &handler.HandleReportsRetrieval{}

	if err := daemon.Initialise("", ""); err != nil {
		t.Fatal(err)
	}
	// Start server and run tests
	serverStopped := make(chan struct{}, 1)
	go func() {
		if err := daemon.StartAndBlockNoTLS(0); err != nil {
			t.Error(err)
			return
		}
		serverStopped <- struct{}{}
	}()
	if !misc.ProbePort(30*time.Second, daemon.Address, daemon.Port) {
		t.Fatalf("server on %s:%d did not start on time", daemon.Address, daemon.Port)
	}
	TestHTTPD(&daemon, t)
	TestAPIHandlers(&daemon, t)

	daemon.StopNoTLS()
	<-serverStopped
	// Repeatedly stopping the daemon should have no negative consequence
	daemon.StopNoTLS()
	daemon.StopNoTLS()
}

func TestHTTPD_IndexPageFromEnv(t *testing.T) {
	daemon := Daemon{
		Address: "localhost",
		Port:    21987,
	}
	os.Setenv(EnvironmentIndexPage, "hi from env")
	if err := daemon.Initialise("/test-prefix", ""); err != nil {
		t.Fatalf("%+v", err)
	}
	go func() {
		if err := daemon.StartAndBlockNoTLS(0); err != nil {
			t.Error(err)
			return
		}
	}()
	if !misc.ProbePort(30*time.Second, daemon.Address, daemon.Port) {
		t.Fatal("server did not start in time")
	}
	serverURLPrefix := fmt.Sprintf("http://localhost:%d/test-prefix", daemon.Port)
	for _, path := range []string{"/", "/index.htm", "/index.html"} {
		resp, err := inet.DoHTTP(context.Background(), inet.HTTPRequest{}, serverURLPrefix+path)
		if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "hi from env") {
			t.Fatal(serverURLPrefix+path, err, string(resp.Body), resp)
		}
	}
	daemon.StopNoTLS()
}
