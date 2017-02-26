package httpd

import (
	"github.com/HouzuoGuo/websh/frontend/common"
	"github.com/HouzuoGuo/websh/frontend/httpd/api"
	"github.com/HouzuoGuo/websh/httpclient"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TODO: upgrade to go 1.8 and implement graceful httpd shutdown.
func TestHTTPD_StartAndBlock(t *testing.T) {
	daemon := HTTPD{
		ListenAddress: "127.0.0.1",
		ListenPort:    13589, // hard coded port is a random choice
		Processor:     &common.CommandProcessor{},
		Handlers: map[string]api.HandlerFactory{
			"/twilio_sms":      &api.TwilioSMSHook{},
			"/twilio_call":     &api.TwilioCallHook{CallbackEndpoint: "/twilio_callback", CallGreeting: "hello"},
			"/twilio_callback": &api.TwilioCallCallback{MyEndpoint: "/twilio_callback"},
			"/self_test":       &api.FeatureSelfTest{},
		},
	}
	if err := daemon.StartAndBlock(); err == nil || !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal("did not error due to insane CommandProcessor")
	}
	daemon.Processor = common.GetTestCommandProcessor()
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)

	addr := "http://127.0.0.1:13589/"
	resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+"self_test")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatal(err, resp)
	}
}
