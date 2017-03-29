package api

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/bridge"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/global"
	"net/http"
)

const TwilioHandlerTimeoutSec = 14 // as of 2017-02-23, the timeout is required by Twilio on both SMS and call hooks.

// Implement handler for Twilio phone number's SMS hook.
type HandleTwilioSMSHook struct {
}

func (hand *HandleTwilioSMSHook) MakeHandler(logger global.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	fun := func(w http.ResponseWriter, r *http.Request) {
		// SMS message is in "Body" parameter
		ret := cmdProc.Process(feature.Command{
			TimeoutSec: TwilioHandlerTimeoutSec,
			Content:    r.FormValue("Body"),
		})
		// In case both PIN and shortcuts mismatch, try to conceal this endpoint.
		if ret.Error == bridge.ErrPINAndShortcutNotFound {
			http.Error(w, "404 page not found", http.StatusNotFound)
		}
		// Generate normal XML response
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "must-revalidate")
		w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response><Message><![CDATA[%s]]></Message></Response>
`, XMLEscape(ret.CombinedOutput))))
	}
	return fun, nil
}
func (hand *HandleTwilioSMSHook) GetRateLimitFactor() int {
	return 1
}

// Implement handler for Twilio phone number's telephone hook.
type HandleTwilioCallHook struct {
	CallGreeting     string `json:"CallGreeting"` // a message to speak upon picking up a call
	CallbackEndpoint string `json:"-"`            // URL (e.g. /handle_my_call) to command handler endpoint (TwilioCallCallback)
}

func (hand *HandleTwilioCallHook) MakeHandler(logger global.Logger, _ *common.CommandProcessor) (http.HandlerFunc, error) {
	if hand.CallGreeting == "" || hand.CallbackEndpoint == "" {
		return nil, errors.New("HandleTwilioCallHook.MakeHandler: greeting or callback endpoint is empty")
	}
	fun := func(w http.ResponseWriter, r *http.Request) {
		// The greeting XML tells Twilio to ask user for DTMF input, and direct the input to another URL endpoint.
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "must-revalidate")
		w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say><![CDATA[%s]]></Say>
    </Gather>
</Response>
`, hand.CallbackEndpoint, XMLEscape(hand.CallGreeting))))
	}
	return fun, nil
}
func (hand *HandleTwilioCallHook) GetRateLimitFactor() int {
	return 10
}

// Implement handler for Twilio phone number's telephone callback (triggered by response of TwilioCallHook).
type HandleTwilioCallCallback struct {
	MyEndpoint string `json:"-"` // URL endpoint to the callback itself, including prefix /.
}

func (hand *HandleTwilioCallCallback) MakeHandler(logger global.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	if hand.MyEndpoint == "" {
		return nil, errors.New("HandleTwilioCallCallback.MakeHandler: own endpoint is empty")
	}
	fun := func(w http.ResponseWriter, r *http.Request) {
		// DTMF input digits are in "Digits" parameter
		ret := cmdProc.Process(feature.Command{
			TimeoutSec: TwilioHandlerTimeoutSec,
			Content:    DTMFDecode(r.FormValue("Digits")),
		})
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
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
        <Say><![CDATA[%s, repeat again, %s, repeat again, %s, over.]]></Say>
    </Gather>
</Response>
`, hand.MyEndpoint, combinedOutput, combinedOutput, combinedOutput)))
		}
	}
	return fun, nil
}

func (hand *HandleTwilioCallCallback) GetRateLimitFactor() int {
	return 1
}
