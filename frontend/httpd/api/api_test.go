package api

import (
	"github.com/HouzuoGuo/websh/frontend/common"
	"github.com/HouzuoGuo/websh/httpclient"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestXMLEscape(t *testing.T) {
	if out := XMLEscape("<!--&ha"); out != "&lt;!--&amp;ha" {
		t.Fatal(out)
	}
}

// TODO: upgrade to go 1.8 and implement graceful httpd shutdown, then break this function apart.
func TestAllHandlers(t *testing.T) {
	// ============ All handlers are tested here ============
	proc := common.GetTestCommandProcessor()

	//  ============ Refuse to make handler function if processor is not sane ============
	var hook HandlerFactory
	hook = &FeatureSelfTest{CommandProcessor: &common.CommandProcessor{}}
	if fun, err := hook.MakeHandler(); err == nil || fun != nil {
		t.Fatal("did not error")
	}
	hook = &TwilioSMSHook{CommandProcessor: &common.CommandProcessor{}}
	if fun, err := hook.MakeHandler(); err == nil || fun != nil {
		t.Fatal("did not error")
	}
	hook = &TwilioCallHook{CommandProcessor: &common.CommandProcessor{}}
	if fun, err := hook.MakeHandler(); err == nil || fun != nil {
		t.Fatal("did not error")
	}
	hook = &TwilioCallCallback{CommandProcessor: &common.CommandProcessor{}}
	if fun, err := hook.MakeHandler(); err == nil || fun != nil {
		t.Fatal("did not error")
	}

	// ============ Give handlers to HTTP server mux ============
	handlers := http.NewServeMux()

	// Self test
	hook = &FeatureSelfTest{CommandProcessor: proc}
	selfTestHandle, err := hook.MakeHandler()
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/self_test", selfTestHandle)
	// Twilio
	hook = &TwilioSMSHook{CommandProcessor: proc}
	smsHandle, err := hook.MakeHandler()
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/sms", smsHandle)
	hook = &TwilioCallHook{CommandProcessor: proc, CallGreeting: "Hi there", CallbackEndpoint: "/test"}
	callHandle, err := hook.MakeHandler()
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/call_greeting", callHandle)
	hook = &TwilioCallCallback{CommandProcessor: proc, MyEndpoint: "/test"}
	callbackHandle, err := hook.MakeHandler()
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/call_command", callbackHandle)

	// ============ Start HTTP server ============
	httpServer := http.Server{Handler: handlers, Addr: "127.0.0.1:34791"} // hard coded port is a random choice
	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)

	// ============ Use HTTP client to test each API ============
	addr := "http://127.0.0.1:34791/"
	// Self test
	resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+"self_test")
	expected := `All OK`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body))
	}
	// Self test - break shell and expect error
	oldShellInterpreter := proc.Features.Shell.InterpreterPath
	proc.Features.Shell.InterpreterPath = ""
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"self_test")
	expected = `map[.s:fork/exec : no such file or directory]`
	if err != nil || resp.StatusCode != http.StatusInternalServerError || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body))
	}
	proc.Features.Shell.InterpreterPath = oldShellInterpreter
	// Twilio
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"Body": {"pin mismatch"}}.Encode()),
	}, addr+"sms")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, resp)
	}
	// Twilio - the extra spaces around prefix and PIN do not matter
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"Body": {"verysecret .s echo 0123456789012345678901234567890123456789"}}.Encode()),
	}, addr+"sms")
	expected = `<?xml version="1.0" encoding="UTF-8"?>
<Response><Message>01234567890123456789012345678901234</Message></Response>
`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, resp)
	}
	// Twilio - check phone call greeting
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"call_greeting")
	expected = `<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="/test" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say>Hi there</Say>
    </Gather>
</Response>
`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, resp)
	}
	// Twilio - check phone call response to DTMF
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"Digits": {"0000000"}}.Encode()),
	}, addr+"call_command")
	expected = `<?xml version="1.0" encoding="UTF-8"?>
<Response>
	<Say>Sorry</Say>
	<Hangup/>
</Response>
`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, resp)
	}
	// Twilio - check phone call response to command
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		//                                            v  e r  y  s   e c  r  e t .   s    tr  u e
		Body: strings.NewReader(url.Values{"Digits": {"88833777999777733222777338014207777087778833"}}.Encode()),
	}, addr+"call_command")
	expected = `<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="/test" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say>EMPTY OUTPUT repeat again, EMPTY OUTPUT repeat again, EMPTY OUTPUT over.</Say>
    </Gather>
</Response>
`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body))
	}
}
