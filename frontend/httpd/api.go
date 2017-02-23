package httpd

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/HouzuoGuo/websh/bridge"
	"github.com/HouzuoGuo/websh/feature"
	"github.com/HouzuoGuo/websh/frontend/common"
	"log"
	"net/http"
)

const TWILIO_HANDLER_TIMEOUT_SEC = 14 // as of 2017-02-23, the timeout is required by Twilio on both SMS and call hooks.

// Escape sequences in a string to make it safe for being element data.
func XMLEscape(in string) string {
	var escapeOutput bytes.Buffer
	if err := xml.EscapeText(&escapeOutput, []byte(in)); err != nil {
		log.Printf("XMLEscape: failed - %v", err)
	}
	return escapeOutput.String()
}

// Create HTTP handler functions for Twilio phone number hook.
type TwilioFactory struct {
	CommandProcessor           *common.CommandProcessor
	CallGreeting               string // a message to speak upon picking up a call
	CallCommandHandlerEndpoint string // URL (e.g. /handle_my_call) to command handler endpoint
}

// Run command from incoming SMS.
func (factory *TwilioFactory) SMSHandler() (http.HandlerFunc, error) {
	if errs := factory.CommandProcessor.IsSaneForInternet(); len(errs) > 0 {
		return nil, fmt.Errorf("%+v", errs)
	}
	fun := func(w http.ResponseWriter, r *http.Request) {
		// SMS message is in "Body" parameter
		ret := factory.CommandProcessor.Process(feature.Command{
			TimeoutSec: TWILIO_HANDLER_TIMEOUT_SEC,
			Content:    r.FormValue("Body"),
		})
		// In case both PIN and shortcuts mismatch, try to conceal this endpoint.
		if ret.Error == bridge.ErrPINAndShortcutNotFound {
			http.Error(w, "404 page not found", http.StatusNotFound)
		}
		// Generate normal XML response
		w.Header().Set("Content-Type", "text/xml")
		w.Header().Set("Cache-Control", "must-revalidate")
		w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response><Message>%s</Message></Response>
`, XMLEscape(ret.CombinedOutput))))
	}
	return fun, nil
}

// Say a greeting message when picking up a call.
func (factory *TwilioFactory) CallGreetingHandler() (http.HandlerFunc, error) {
	if errs := factory.CommandProcessor.IsSaneForInternet(); len(errs) > 0 {
		return nil, fmt.Errorf("%+v", errs)
	}
	fun := func(w http.ResponseWriter, r *http.Request) {
		// The greeting XML tells Twilio to ask user for DTMF input, and direct the input to another URL endpoint.
		w.Header().Set("Content-Type", "text/xml")
		w.Header().Set("Cache-Control", "must-revalidate")
		w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say>%s</Say>
    </Gather>
</Response>
`, factory.CallCommandHandlerEndpoint, XMLEscape(factory.CallGreeting))))
	}
	return fun, nil
}

// Run command from DTMF input and wait for next command.
func (factory *TwilioFactory) CallCommandHandler() (http.HandlerFunc, error) {
	if errs := factory.CommandProcessor.IsSaneForInternet(); len(errs) > 0 {
		return nil, fmt.Errorf("%+v", errs)
	}
	fun := func(w http.ResponseWriter, r *http.Request) {
		// DTMF input digits are in "Digits" parameter
		ret := factory.CommandProcessor.Process(feature.Command{
			TimeoutSec: TWILIO_HANDLER_TIMEOUT_SEC,
			Content:    DTMFDecode(r.FormValue("Digits")),
		})
		w.Header().Set("Content-Type", "text/xml")
		w.Header().Set("Cache-Control", "must-revalidate")
		// Say sorry and hang up in case of incorrect PIN/shortcut
		if ret.Error == bridge.ErrPINAndShortcutNotFound {
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
	<Say>Sorry</Say>
	<Hangup/>
</Response>
`))
		} else {
			// Repeat output three times and listen for the next input
			combinedOutput := XMLEscape(ret.CombinedOutput)
			w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say>%s repeat again, %s repeat again, %s over.</Say>
    </Gather>
</Response>
`, factory.CallCommandHandlerEndpoint, combinedOutput, combinedOutput, combinedOutput)))
		}
	}
	return fun, nil
}
