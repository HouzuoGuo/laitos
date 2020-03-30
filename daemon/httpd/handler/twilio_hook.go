package handler

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const (
	/*
		TwilioHandlerTimeoutSec is Twilio's HTTP client timeout. As of 2017-02-23, the timeout of 14 seconds is enforced
		on both SMS and telephone call hooks. To leave some room for IO transfer, the command timeout takes away two
		seconds from the absolute timeout of 14 seconds.
	*/
	TwilioHandlerTimeoutSec = 14 - 2
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
	TwilioAPIRateLimitFactor = 16

	/*
		TwilioPhoneNumberRateLimitIntervalSec is an interval measured in number of seconds that an incoming phone number is
		allowed to invoke SMS or voice call routine. This rate limit is designed to prevent spam SMS and calls.
	*/
	TwilioPhoneNumberRateLimitIntervalSec = 5
)

// Handle Twilio phone number's SMS hook.
type HandleTwilioSMSHook struct {
	senderRateLimit *misc.RateLimit // senderRateLimit prevents excessive SMS replies from being replied to spam numbers

	logger  lalog.Logger
	cmdProc *toolbox.CommandProcessor
}

func (hand *HandleTwilioSMSHook) Initialise(logger lalog.Logger, cmdProc *toolbox.CommandProcessor) error {
	hand.logger = logger
	hand.cmdProc = cmdProc
	// Allow maximum of 1 SMS to be received every 5 seconds, per phone number.
	hand.senderRateLimit = &misc.RateLimit{
		UnitSecs: TwilioPhoneNumberRateLimitIntervalSec,
		MaxCount: 1,
		Logger:   logger,
	}
	hand.senderRateLimit.Initialise()
	return nil
}

func (hand *HandleTwilioSMSHook) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	NoCache(w)
	// Apply rate limit to the sender
	phoneNumber := r.FormValue("From")
	hand.logger.Info("HandleTwilioSMSHook", phoneNumber, nil, "has received an SMS")
	if phoneNumber != "" {
		if !hand.senderRateLimit.Add(phoneNumber, true) {
			/*
				Twilio does not have a reject feature for incoming SMS, therefore, use a non-2xx HTTP status code to
				inform Twilio not to make an SMS reply. Twilio user will see the the rate limit error message in Twilio
				console.
			*/
			http.Error(w, "rate limit is exceeded by sender "+phoneNumber, http.StatusServiceUnavailable)
			return
		}
	}
	// SMS message is in "Body" parameter
	ret := hand.cmdProc.Process(toolbox.Command{
		DaemonName: "httpd",
		ClientID:   phoneNumber,
		TimeoutSec: TwilioHandlerTimeoutSec,
		Content:    r.FormValue("Body"),
	}, true)
	// Generate normal XML response
	_, _ = w.Write([]byte(fmt.Sprintf(xml.Header+`
<Response><Message><![CDATA[%s]]></Message></Response>
`, XMLEscape(ret.CombinedOutput))))
}
func (hand *HandleTwilioSMSHook) GetRateLimitFactor() int {
	return TwilioAPIRateLimitFactor
}
func (_ *HandleTwilioSMSHook) SelfTest() error {
	return nil
}

// Say a greeting in Twilio phone number's telephone call hook.
type HandleTwilioCallHook struct {
	CallGreeting     string `json:"CallGreeting"` // a message to speak upon picking up a call
	CallbackEndpoint string `json:"-"`            // URL (e.g. /handle_my_call) to command handler endpoint (TwilioCallCallback)

	senderRateLimit *misc.RateLimit // senderRateLimit prevents excessive calls from being made by spam numbers
	logger          lalog.Logger
	cmdProc         *toolbox.CommandProcessor
}

func (hand *HandleTwilioCallHook) Initialise(logger lalog.Logger, cmdProc *toolbox.CommandProcessor) error {
	if hand.CallGreeting == "" || hand.CallbackEndpoint == "" {
		return errors.New("HandleTwilioCallHook.Initialise: greeting and callback endpoint must not be empty")
	}
	hand.logger = logger
	hand.cmdProc = cmdProc
	// Allows maximum of 1 call to be received every 5 seconds
	hand.senderRateLimit = &misc.RateLimit{
		UnitSecs: TwilioPhoneNumberRateLimitIntervalSec,
		MaxCount: 1,
		Logger:   logger,
	}
	hand.senderRateLimit.Initialise()
	return nil
}

func (hand *HandleTwilioCallHook) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	NoCache(w)
	// Apply rate limit to the caller
	phoneNumber := r.FormValue("From")
	hand.logger.Info("HandleTwilioCallHook", phoneNumber, nil, "has received a call")
	if phoneNumber != "" {
		if !hand.senderRateLimit.Add(phoneNumber, true) {
			_, _ = w.Write([]byte(xml.Header + `<Response><Reject/></Response>`))
			return
		}
	}
	// The greeting XML tells Twilio to ask user for DTMF input, and direct the input to another URL endpoint.
	_, _ = w.Write([]byte(fmt.Sprintf(xml.Header+`
<Response>
    <Gather action="%s" method="POST" timeout="60" finishOnKey="#" numDigits="1000">
        <Say><![CDATA[%s]]></Say>
    </Gather>
</Response>
`, hand.CallbackEndpoint, XMLEscape(hand.CallGreeting))))
}
func (hand *HandleTwilioCallHook) GetRateLimitFactor() int {
	return TwilioAPIRateLimitFactor
}
func (_ *HandleTwilioCallHook) SelfTest() error {
	return nil
}

// Carry on with command processing in Twilio telephone call conversation.
type HandleTwilioCallCallback struct {
	MyEndpoint string `json:"-"` // URL endpoint to the callback itself, including prefix /.

	senderRateLimit *misc.RateLimit // senderRateLimit prevents excessive calls from being made by spam numbers
	logger          lalog.Logger
	cmdProc         *toolbox.CommandProcessor
}

func (hand *HandleTwilioCallCallback) Initialise(logger lalog.Logger, cmdProc *toolbox.CommandProcessor) error {
	if hand.MyEndpoint == "" {
		return errors.New("HandleTwilioCallCallback.Initialise: MyEndpoint must not be empty")
	}
	hand.logger = logger
	hand.cmdProc = cmdProc
	// Allows maximum of 1 DTMF command to be received every 5 seconds
	hand.senderRateLimit = &misc.RateLimit{
		UnitSecs: TwilioPhoneNumberRateLimitIntervalSec,
		MaxCount: 1,
		Logger:   logger,
	}
	hand.senderRateLimit.Initialise()
	return nil
}

func (hand *HandleTwilioCallCallback) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	NoCache(w)
	// Apply rate limit to the caller
	phoneNumber := r.FormValue("From")
	hand.logger.Info("HandleTwilioCallCallback", phoneNumber, nil, "has received DTMF command via call")
	if phoneNumber != "" {
		if !hand.senderRateLimit.Add(phoneNumber, true) {
			_, _ = w.Write([]byte(xml.Header + `<Response><Say>You are rate limited.</Say><Hangup/></Response>`))
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
	ret := hand.cmdProc.Process(toolbox.Command{
		DaemonName: "httpd",
		ClientID:   phoneNumber,
		TimeoutSec: TwilioHandlerTimeoutSec,
		Content:    toolbox.DTMFDecode(dtmfInput),
	}, true)
	combinedOutput := ret.CombinedOutput
	if phoneticSpelling {
		combinedOutput = toolbox.SpellPhonetically(combinedOutput)
	}
	combinedOutput = XMLEscape(combinedOutput)
	// Repeat command output three times and listen for the next input
	_, _ = w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say><![CDATA[%s.

    repeat again.    

%s.

    repeat again.    

%s.
over.]]></Say>
    </Gather>
</Response>
`, hand.MyEndpoint, combinedOutput, combinedOutput, combinedOutput)))
}

func (hand *HandleTwilioCallCallback) GetRateLimitFactor() int {
	return TwilioAPIRateLimitFactor
}
func (_ *HandleTwilioCallCallback) SelfTest() error {
	return nil
}
