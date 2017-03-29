package api

import (
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/global"
	"github.com/HouzuoGuo/laitos/httpclient"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
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
	logger := global.Logger{}

	// ============ Give handlers to HTTP server mux ============
	handlers := http.NewServeMux()

	var handle HandlerFactory
	// System info
	handle = &HandleSystemInfo{}
	infoHandler, err := handle.MakeHandler(logger, proc)
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/info", infoHandler)
	// Command form
	handle = &HandleCommandForm{}
	cmdFormHandle, err := handle.MakeHandler(logger, proc)
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/cmd_form", cmdFormHandle)
	// Gitlab browser
	handle = &HandleGitlabBrowser{PrivateToken: "TestToken"}
	gitlabHandle, err := handle.MakeHandler(logger, proc)
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/gitlab", gitlabHandle)
	// HTML document
	// Create a temporary html file
	indexFile := "/tmp/test-laitos-index.html"
	defer os.Remove(indexFile)
	if err := ioutil.WriteFile(indexFile, []byte("this is index #LAITOS_CLIENTADDR #LAITOS_3339TIME"), 0644); err != nil {
		t.Fatal(err)
	}
	handle = &HandleHTMLDocument{HTMLFilePath: indexFile}
	htmlDocHandle, err := handle.MakeHandler(logger, proc)
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/html", htmlDocHandle)
	// MailMe
	handle = &HandleMailMe{
		Recipients: []string{"howard@localhost"},
		Mailer: email.Mailer{
			MailFrom: "howard@localhost",
			MTAHost:  "localhost",
			MTAPort:  25,
		},
	}
	mailMeHandle, err := handle.MakeHandler(logger, proc)
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/mail_me", mailMeHandle)
	// Proxy
	handle = &HandleWebProxy{MyEndpoint: "/proxy"}
	proxyHandle, err := handle.MakeHandler(logger, proc)
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/proxy", proxyHandle)
	// Twilio
	handle = &HandleTwilioSMSHook{}
	smsHandle, err := handle.MakeHandler(logger, proc)
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/sms", smsHandle)
	handle = &HandleTwilioCallHook{CallGreeting: "Hi there", CallbackEndpoint: "/test"}
	callHandle, err := handle.MakeHandler(logger, proc)
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/call_greeting", callHandle)
	handle = &HandleTwilioCallCallback{MyEndpoint: "/test"}
	callbackHandle, err := handle.MakeHandler(logger, proc)
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
	// System information
	resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+"info")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "Stack traces:") {
		t.Fatal(err, string(resp.Body))
	}
	// Break shell and expect error from system information
	oldShellInterpreter := proc.Features.Shell.InterpreterPath
	proc.Features.Shell.InterpreterPath = ""
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"info")
	errMsg := ".s: fork/exec : no such file or directory"
	if err != nil || resp.StatusCode != http.StatusInternalServerError || strings.Index(string(resp.Body), errMsg) == -1 {
		t.Fatal(err, "\n", string(resp.Body))
	}
	proc.Features.Shell.InterpreterPath = oldShellInterpreter
	// Command Form
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{Method: http.MethodPost}, addr+"cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"cmd": {"verysecret.sls /"}}.Encode()),
	}, addr+"cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "bin") {
		t.Fatal(err, string(resp.Body))
	}
	// Gitlab handle
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"gitlab")
	if err != nil || resp.StatusCode != http.StatusOK || strings.Index(string(resp.Body), "Enter path to browse") == -1 {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Index
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"html")
	expected := "this is index 127.0.0.1 " + time.Now().Format(time.RFC3339)
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body), expected, resp)
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
	// Proxy (visit /html)
	// Normally the proxy should inject javascript into the page, but the home page does not look like HTML so proxy won't do that.
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"proxy?u=http%%3A%%2F%%2F127.0.0.1%%3A34791%%2Fhtml")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.HasPrefix(string(resp.Body), "this is index") {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - exchange SMS with bad PIN
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
<Response><Message><![CDATA[01234567890123456789012345678901234]]></Message></Response>
`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, resp)
	}
	// Twilio - check phone call greeting
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"call_greeting")
	expected = `<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="/test" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say><![CDATA[Hi there]]></Say>
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
        <Say><![CDATA[EMPTY OUTPUT, repeat again, EMPTY OUTPUT, repeat again, EMPTY OUTPUT, over.]]></Say>
    </Gather>
</Response>
`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body))
	}
}
