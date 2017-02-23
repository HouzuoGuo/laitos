package httpd

import (
	"github.com/HouzuoGuo/websh/frontend/common"
	"github.com/HouzuoGuo/websh/frontend/httpd/api"
	"github.com/HouzuoGuo/websh/httpclient"
	"net/http"
	"testing"
	"time"
)

// TODO: upgrade to go 1.8 and implement graceful httpd shutdown.
func TestHTTPD_StartAndBlock(t *testing.T) {
	proc := common.GetTestCommandProcessor()
	daemon := HTTPD{
		ListenAddress: "127.0.0.1",
		ListenPort:    13589, // hard coded port is a random choice
		Handlers: map[string]api.HandlerFactory{
			"/twilio_sms":      &api.TwilioSMSHook{CommandProcessor: proc},
			"/twilio_call":     &api.TwilioCallHook{CommandProcessor: proc, CallbackEndpoint: "/twilio_callback", CallGreeting: "hello"},
			"/twilio_callback": &api.TwilioCallCallback{CommandProcessor: proc, MyEndpoint: "/twilio_callback"},
			"/self_test":       &api.FeatureSelfTest{CommandProcessor: proc},
		},
	}
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
