package api

import (
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/toolbox"
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

	/*
		TwilioAPIRateLimitFactor allows (API rate limit factor * BaseRateLimit) number of requests to be made by Twilio platform
		per HTTP server rate limit interval. Be aware that API handlers place an extra rate limit based on incoming phone number.
		This rate limit is designed to protect brute force PIN attack from accidentally exposed API handler URL.
	*/
	TwilioAPIRateLimitFactor = 10

	/*
		TwilioPhoneNumberRateLimitIntervalSec is an interval measured in number of seconds that an incoming phone number is
		allowed to invoke SMS or voice call routine. This rate limit is designed to prevent spam SMS and calls.
	*/
	TwilioPhoneNumberRateLimitIntervalSec = 5
)

// Handle Twilio phone number's SMS hook.
type HandleTwilioSMSHook struct {
	senderRateLimit *misc.RateLimit // senderRateLimit prevents excessive SMS replies from being replied to spam numbers
}

func (hand *HandleTwilioSMSHook) MakeHandler(logger misc.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	// Allows maximum of 1 SMS to be received every 5 seconds
	hand.senderRateLimit = &misc.RateLimit{
		UnitSecs: TwilioPhoneNumberRateLimitIntervalSec,
		MaxCount: 1,
		Logger:   logger,
	}
	hand.senderRateLimit.Initialise()
	fun := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		NoCache(w)
		if !WarnIfNoHTTPS(r, w) {
			return
		}
		// Apply rate limit to the sender
		phoneNumber := r.FormValue("From")
		logger.Printf("HandleTwilioSMSHook", phoneNumber, nil, "has received an SMS")
		if phoneNumber != "" {
			if !hand.senderRateLimit.Add(phoneNumber, true) {
				/*
					Twilio does not have a reject feature for incoming SMS, therefore, use a non-2xx
					HTTP status code to inform Twilio not to make an SMS reply.
					Twilio user will see the the rate limit error message in Twilio console.
				*/
				http.Error(w, "rate limit is exceeded by "+phoneNumber, http.StatusServiceUnavailable)
				return
			}
		}
		// SMS message is in "Body" parameter
		ret := cmdProc.Process(toolbox.Command{
			TimeoutSec: TwilioHandlerTimeoutSec,
			Content:    r.FormValue("Body"),
		})
		// Generate normal XML response
		w.Write([]byte(fmt.Sprintf(xml.Header+`
<Response><Message><![CDATA[%s]]></Message></Response>
`, XMLEscape(ret.CombinedOutput))))
	}
	return fun, nil
}
func (hand *HandleTwilioSMSHook) GetRateLimitFactor() int {
	return TwilioAPIRateLimitFactor
}

// Say a greeting in Twilio phone number's telephone call hook.
type HandleTwilioCallHook struct {
	CallGreeting     string `json:"CallGreeting"` // a message to speak upon picking up a call
	CallbackEndpoint string `json:"-"`            // URL (e.g. /handle_my_call) to command handler endpoint (TwilioCallCallback)

	senderRateLimit *misc.RateLimit // senderRateLimit prevents excessive calls from being made by spam numbers
}

func (hand *HandleTwilioCallHook) MakeHandler(logger misc.Logger, _ *common.CommandProcessor) (http.HandlerFunc, error) {
	// Allows maximum of 1 call to be received every 5 seconds
	hand.senderRateLimit = &misc.RateLimit{
		UnitSecs: TwilioPhoneNumberRateLimitIntervalSec,
		MaxCount: 1,
		Logger:   logger,
	}
	hand.senderRateLimit.Initialise()
	if hand.CallGreeting == "" || hand.CallbackEndpoint == "" {
		return nil, errors.New("HandleTwilioCallHook.MakeHandler: greeting or callback endpoint is empty")
	}
	fun := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		NoCache(w)
		if !WarnIfNoHTTPS(r, w) {
			return
		}
		// Apply rate limit to the caller
		phoneNumber := r.FormValue("From")
		logger.Printf("HandleTwilioCallHook", phoneNumber, nil, "has received a call")
		if phoneNumber != "" {
			if !hand.senderRateLimit.Add(phoneNumber, true) {
				w.Write([]byte(xml.Header + `<Response><Reject/></Response>`))
				return
			}
		}
		// The greeting XML tells Twilio to ask user for DTMF input, and direct the input to another URL endpoint.
		w.Write([]byte(fmt.Sprintf(xml.Header+`
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
	return TwilioAPIRateLimitFactor
}

// Carry on with command processing in Twilio telephone call conversation.
type HandleTwilioCallCallback struct {
	MyEndpoint string `json:"-"` // URL endpoint to the callback itself, including prefix /.

	senderRateLimit *misc.RateLimit // senderRateLimit prevents excessive calls from being made by spam numbers
}

func (hand *HandleTwilioCallCallback) MakeHandler(logger misc.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	// Allows maximum of 1 DTMF command to be received every 5 seconds
	hand.senderRateLimit = &misc.RateLimit{
		UnitSecs: TwilioPhoneNumberRateLimitIntervalSec,
		MaxCount: 1,
		Logger:   logger,
	}
	hand.senderRateLimit.Initialise()
	if hand.MyEndpoint == "" {
		return nil, errors.New("HandleTwilioCallCallback.MakeHandler: own endpoint is empty")
	}
	fun := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		NoCache(w)
		if !WarnIfNoHTTPS(r, w) {
			return
		}
		// Apply rate limit to the caller
		phoneNumber := r.FormValue("From")
		logger.Printf("HandleTwilioCallHook", phoneNumber, nil, "has received DTMF command via call")
		if phoneNumber != "" {
			if !hand.senderRateLimit.Add(phoneNumber, true) {
				w.Write([]byte(xml.Header + `<Response><Say>You are rate limited.</Say><Hangup/></Response>`))
				return
			}
		}
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
	return fun, nil
}

func (hand *HandleTwilioCallCallback) GetRateLimitFactor() int {
	return TwilioAPIRateLimitFactor
}
