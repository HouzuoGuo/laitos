package api

import (
	"github.com/HouzuoGuo/websh/email"
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

	// ============ Give handlers to HTTP server mux ============
	handlers := http.NewServeMux()

	// Self test
	var handle HandlerFactory = &HandleFeatureSelfTest{}
	selfTestHandle, err := handle.MakeHandler(proc)
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/self_test", selfTestHandle)
	// Twilio
	handle = &HandleTwilioSMSHook{}
	smsHandle, err := handle.MakeHandler(proc)
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/sms", smsHandle)
	handle = &HandleTwilioCallHook{CallGreeting: "Hi there", CallbackEndpoint: "/test"}
	callHandle, err := handle.MakeHandler(proc)
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/call_greeting", callHandle)
	handle = &HandleTwilioCallCallback{MyEndpoint: "/test"}
	callbackHandle, err := handle.MakeHandler(proc)
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/call_command", callbackHandle)
	// MailMe
	handle = &HandleMailMe{
		Recipients: []string{"howard@localhost"},
		Mailer: &email.Mailer{
			MailFrom: "howard@localhost",
			MTAHost:  "localhost",
			MTAPort:  25,
		},
	}
	mailMeHandle, err := handle.MakeHandler(proc)
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/mail_me", mailMeHandle)

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
	// Twilio - exchange SMS, the extra spaces around prefix and PIN do not matter.
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
		//                                             v  e r  y  s   e c  r  e t .   s    tr  u e
		Body: strings.NewReader(url.Values{"Digits": {"88833777999777733222777338014207777087778833"}}.Encode()),
	}, addr+"call_command")
	expected = `<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="/test" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say>EMPTY OUTPUT, repeat again, EMPTY OUTPUT, repeat again, EMPTY OUTPUT, over.</Say>
    </Gather>
</Response>
`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body))
	}
	// MailMe
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"mail_me")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{Method: http.MethodPost}, addr+"mail_me")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"msg": {"又给你发了一个邮件"}}.Encode()),
	}, addr+"mail_me")
	if err != nil || resp.StatusCode != http.StatusOK ||
		(!strings.Contains(string(resp.Body), "发不出去") && !strings.Contains(string(resp.Body), "发出去了")) {
		t.Fatal(err, string(resp.Body))
	}
}
