package api

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/HouzuoGuo/laitos/toolbox/filter"
	"net/http"
	"strings"
)

const (
	/*
		TwilioHandlerTimeoutSec is Twilio's HTTP client timeout. As of 2017-02-23, the timeout of 14 seconds is enforced
		on both SMS and telephone call hooks.
	*/
	TwilioHandlerTimeoutSec = 14
	/*
		TwilioPhoneticSpellingMagic is the prefix magic to dial in order to have output read back phonetically.
		The magic must not hinder DTMF PIN input, therefore it begins with 0, which is a DTMF space and cannot be part of a PIN.
	*/
	TwilioPhoneticSpellingMagic = "0123"
)

// Handle Twilio phone number's SMS hook.
type HandleTwilioSMSHook struct {
}

func (hand *HandleTwilioSMSHook) MakeHandler(logger misc.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	fun := func(w http.ResponseWriter, r *http.Request) {
		// SMS message is in "Body" parameter
		ret := cmdProc.Process(toolbox.Command{
			TimeoutSec: TwilioHandlerTimeoutSec,
			Content:    r.FormValue("Body"),
		})
		// In case both PIN and shortcuts mismatch, try to conceal this endpoint.
		if ret.Error == filter.ErrPINAndShortcutNotFound {
			http.Error(w, "404 page not found", http.StatusNotFound)
		}
		// Generate normal XML response
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		NoCache(w)
		if !WarnIfNoHTTPS(r, w) {
			return
		}
		w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response><Message><![CDATA[%s]]></Message></Response>
`, XMLEscape(ret.CombinedOutput))))
	}
	return fun, nil
}
func (hand *HandleTwilioSMSHook) GetRateLimitFactor() int {
	return 1
}

// Say a greeting in Twilio phone number's telephone call hook.
type HandleTwilioCallHook struct {
	CallGreeting     string `json:"CallGreeting"` // a message to speak upon picking up a call
	CallbackEndpoint string `json:"-"`            // URL (e.g. /handle_my_call) to command handler endpoint (TwilioCallCallback)
}

func (hand *HandleTwilioCallHook) MakeHandler(logger misc.Logger, _ *common.CommandProcessor) (http.HandlerFunc, error) {
	if hand.CallGreeting == "" || hand.CallbackEndpoint == "" {
		return nil, errors.New("HandleTwilioCallHook.MakeHandler: greeting or callback endpoint is empty")
	}
	fun := func(w http.ResponseWriter, r *http.Request) {
		// The greeting XML tells Twilio to ask user for DTMF input, and direct the input to another URL endpoint.
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		NoCache(w)
		if !WarnIfNoHTTPS(r, w) {
			return
		}
		w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s" method="POST" timeout="60" finishOnKey="#" numDigits="1000">
        <Say><![CDATA[%s]]></Say>
    </Gather>
</Response>
`, hand.CallbackEndpoint, XMLEscape(hand.CallGreeting))))
	}
	return fun, nil
}
func (hand *HandleTwilioCallHook) GetRateLimitFactor() int {
	return 1
}

// Carry on with command processing in Twilio telephone call conversation.
type HandleTwilioCallCallback struct {
	MyEndpoint string `json:"-"` // URL endpoint to the callback itself, including prefix /.
}

func (hand *HandleTwilioCallCallback) MakeHandler(logger misc.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	if hand.MyEndpoint == "" {
		return nil, errors.New("HandleTwilioCallCallback.MakeHandler: own endpoint is empty")
	}
	fun := func(w http.ResponseWriter, r *http.Request) {
		// DTMF input digits are in "Digits" parameter
		var phoneticSpelling bool
		dtmfInput := r.FormValue("Digits")
		// The magic prefix asks output to be spelt phonetically
		if strings.HasPrefix(dtmfInput, TwilioPhoneticSpellingMagic) {
			phoneticSpelling = true
			dtmfInput = dtmfInput[len(TwilioPhoneticSpellingMagic):]
		}
		// Run the toolbox command
		ret := cmdProc.Process(toolbox.Command{
			TimeoutSec: TwilioHandlerTimeoutSec,
			Content:    DTMFDecode(dtmfInput),
		})
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		NoCache(w)
		if !WarnIfNoHTTPS(r, w) {
			return
		}
		// Say sorry and hang up in case of incorrect PIN/shortcut
		if ret.Error == filter.ErrPINAndShortcutNotFound {
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
	<Say>Sorry</Say>
	<Hangup/>
</Response>
`))
		} else {
			combinedOutput := ret.CombinedOutput
			if phoneticSpelling {
				combinedOutput = SpellPhonetically(combinedOutput)
			}
			combinedOutput = XMLEscape(combinedOutput)
			// Repeat command output three times and listen for the next input
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
