package httpd

import (
	"github.com/HouzuoGuo/websh/bridge"
	"github.com/HouzuoGuo/websh/feature"
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

func TestTwilioFactory(t *testing.T) {
	// Prepare feature set - the shell execution feature should be available even without configuration
	features := &feature.FeatureSet{}
	if err := features.Initialise(); err != nil {
		t.Fatal(features)
	}
	// Prepare realistic command bridges
	commandBridges := []bridge.CommandBridge{
		&bridge.CommandPINOrShortcut{PIN: "verysecret"},
		&bridge.CommandTranslator{Sequences: [][]string{{"alpha", "beta"}}},
	}
	// Prepare realistic result bridges
	resultBridges := []bridge.ResultBridge{
		&bridge.ResetCombinedText{},
		&bridge.LintCombinedText{TrimSpaces: true, MaxLength: 35},
		&bridge.SayEmptyOutput{},
		&bridge.NotifyViaEmail{},
	}
	// Prepare a good command processor
	proc := &common.CommandProcessor{
		Features:       features,
		CommandBridges: commandBridges,
		ResultBridges:  resultBridges,
	}

	// Factory should refuse to make handler functions if processor isn't sane
	factory := TwilioFactory{CallCommandHandlerEndpoint: "/test", CallGreeting: "Hi there", CommandProcessor: &common.CommandProcessor{}}
	if fun, err := factory.CallCommandHandler(); err == nil || fun != nil {
		t.Fatal("did not error")
	}
	if fun, err := factory.CallGreetingHandler(); err == nil || fun != nil {
		t.Fatal("did not error")
	}
	if fun, err := factory.SMSHandler(); err == nil || fun != nil {
		t.Fatal("did not error")
	}

	// Make an HTTP server and test handler functions
	factory = TwilioFactory{CallCommandHandlerEndpoint: "/test", CallGreeting: "Hi there", CommandProcessor: proc}
	handlers := http.NewServeMux()
	greetingHandle, err := factory.CallGreetingHandler()
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/call_greeting", greetingHandle)
	commandHandle, err := factory.CallCommandHandler()
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/call_command", commandHandle)
	smsHandle, err := factory.SMSHandler()
	if err != nil {
		t.Fatal(err)
	}
	handlers.HandleFunc("/sms", smsHandle)
	httpServer := http.Server{Handler: handlers, Addr: "127.0.0.1:34791"}
	// Server must start within a second
	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(1 * time.Second)

	// Check server APIs with an HTTP client
	addr := "http://127.0.0.1:34791/"
	resp, err := httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"Body": {"pin mismatch"}}.Encode()),
	}, addr+"sms")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, resp)
	}
	// The extra spaces around prefix and PIN do not matter
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"Body": {"verysecret .s echo 0123456789012345678901234567890123456789"}}.Encode()),
	}, addr+"sms")
	expected := `<?xml version="1.0" encoding="UTF-8"?>
<Response><Message>01234567890123456789012345678901234</Message></Response>
`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, resp)
	}
	// Check phone call greeting
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
	// Check phone call response to DTMF
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
	// Check phone call response to command
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
