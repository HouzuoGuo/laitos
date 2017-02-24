package api

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/websh/bridge"
	"github.com/HouzuoGuo/websh/feature"
	"github.com/HouzuoGuo/websh/frontend/common"
	"net/http"
)

const TWILIO_HANDLER_TIMEOUT_SEC = 14 // as of 2017-02-23, the timeout is required by Twilio on both SMS and call hooks.

// Implement handler for Twilio phone number's SMS hook.
type TwilioSMSHook struct {
	CommandProcessor *common.CommandProcessor
}

func (hand *TwilioSMSHook) MakeHandler() (http.HandlerFunc, error) {
	if errs := hand.CommandProcessor.IsSaneForInternet(); len(errs) > 0 {
		return nil, fmt.Errorf("%+v", errs)
	}
	fun := func(w http.ResponseWriter, r *http.Request) {
		// SMS message is in "Body" parameter
		ret := hand.CommandProcessor.Process(feature.Command{
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

// Implement handler for Twilio phone number's telephone hook.
type TwilioCallHook struct {
	CommandProcessor *common.CommandProcessor
	CallGreeting     string // a message to speak upon picking up a call
	CallbackEndpoint string // URL (e.g. /handle_my_call) to command handler endpoint (TwilioCallCallback)
}

func (hand *TwilioCallHook) MakeHandler() (http.HandlerFunc, error) {
	if errs := hand.CommandProcessor.IsSaneForInternet(); len(errs) > 0 {
		return nil, fmt.Errorf("%+v", errs)
	}
	if hand.CallGreeting == "" || hand.CallbackEndpoint == "" {
		return nil, errors.New("Greeting or handler endpoint is empty")
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
`, hand.CallbackEndpoint, XMLEscape(hand.CallGreeting))))
	}
	return fun, nil
}

// Implement handler for Twilio phone number's telephone callback (triggered by response of TwilioCallHook).
type TwilioCallCallback struct {
	CommandProcessor *common.CommandProcessor
	MyEndpoint       string // URL to the callback itself
}

func (hand *TwilioCallCallback) MakeHandler() (http.HandlerFunc, error) {
	if errs := hand.CommandProcessor.IsSaneForInternet(); len(errs) > 0 {
		return nil, fmt.Errorf("%+v", errs)
	}
	if hand.MyEndpoint == "" {
		return nil, errors.New("Handler endpoint is empty")
	}
	fun := func(w http.ResponseWriter, r *http.Request) {
		// DTMF input digits are in "Digits" parameter
		ret := hand.CommandProcessor.Process(feature.Command{
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
        <Say>%s, repeat again, %s, repeat again, %s, over.</Say>
    </Gather>
</Response>
`, hand.MyEndpoint, combinedOutput, combinedOutput, combinedOutput)))
		}
	}
	return fun, nil
}
