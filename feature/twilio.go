package feature

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/websh/httpclient"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	TwilioMakeCall = "c"
	TwilioSendSMS  = "t"
)

var RegexNumberAndMessage = regexp.MustCompile(`(\+[0-9]+)[^\w]+(.*)`)

type Twilio struct {
	PhoneNumber string `json:"PhoneNumber"` // Twilio telephone country code and number (the number you purchased from Twilio)
	AccountSID  string `json:"AccountSID"`  // Twilio account SID ("Account Settings - LIVE Credentials - Account SID")
	AuthToken   string `json:"AuthToken"`   // Twilio authentication secret token ("Account Settings - LIVE Credentials - Auth Token")

	TestPhoneNumber string `json:"-"` // Set by init_test.go for running test case, not a configuration.
}

var TestTwilio = Twilio{} // API credentials are set by init_test.go

func (twi *Twilio) IsConfigured() bool {
	return twi.PhoneNumber != "" && twi.AccountSID != "" && twi.AuthToken != ""
}

func (twi *Twilio) SelfTest() error {
	if !twi.IsConfigured() {
		return ErrIncompleteConfig
	}
	// Validate API credentials with a simple API call
	resp, err := httpclient.DoHTTP(httpclient.Request{
		TimeoutSec: FeatureTestTimeoutSec,
		RequestFunc: func(req *http.Request) error {
			req.SetBasicAuth(twi.AccountSID, twi.AuthToken)
			return nil
		},
	}, "https://api.twilio.com/2010-04-01/Accounts/%s", twi.AccountSID)
	if err != nil {
		return err
	}
	return resp.Non2xxToError()
}

func (twi *Twilio) Initialise() error {
	return nil
}

func (twi *Twilio) Trigger() Trigger {
	return ".p"
}

func (twi *Twilio) Execute(cmd Command) (ret *Result) {
	if errResult := cmd.Trim(); errResult != nil {
		ret = errResult
		return
	}

	if strings.HasPrefix(cmd.Content, TwilioMakeCall) {
		ret = twi.MakeCall(cmd)
	} else if strings.HasPrefix(cmd.Content, TwilioSendSMS) {
		ret = twi.SendSMS(cmd)
	} else {
		ret = &Result{Error: fmt.Errorf("Failed to find command prefix (either %s or %s)", TwilioMakeCall, TwilioSendSMS)}
	}
	return
}

func (twi *Twilio) MakeCall(cmd Command) *Result {
	params := RegexNumberAndMessage.FindStringSubmatch(strings.TrimSpace(strings.TrimPrefix(cmd.Content, TwilioMakeCall)))
	if len(params) < 3 {
		return &Result{Error: errors.New("Call parameters are missing")}
	}
	toNumber := params[1]
	message := params[2]
	formParams := url.Values{
		"From": {twi.PhoneNumber},
		"To":   {toNumber},
		"Url":  {"http://twimlets.com/message?Message=" + url.QueryEscape(fmt.Sprintf("%s, repeat again, %s, repeat again, %s, over.", message, message, message))},
	}
	resp, err := httpclient.DoHTTP(httpclient.Request{
		TimeoutSec: FeatureTestTimeoutSec,
		Method:     http.MethodPost,
		Body:       strings.NewReader(formParams.Encode()),
		RequestFunc: func(req *http.Request) error {
			req.SetBasicAuth(twi.AccountSID, twi.AuthToken)
			return nil
		},
	}, "https://api.twilio.com/2010-04-01/Accounts/%s/Calls.json", twi.AccountSID)
	if errResult := HTTPErrorToResult(resp, err); errResult != nil {
		return errResult
	}
	// The OK output is simply the length of number + message
	return &Result{Error: nil, Output: strconv.Itoa(len(toNumber) + len(message))}
}

func (twi *Twilio) SendSMS(cmd Command) *Result {
	params := RegexNumberAndMessage.FindStringSubmatch(strings.TrimSpace(strings.TrimPrefix(cmd.Content, TwilioMakeCall)))
	if len(params) < 3 {
		return &Result{Error: errors.New("SMS parameters are missing")}
	}
	toNumber := params[1]
	message := params[2]

	formParams := url.Values{
		"From": {twi.PhoneNumber},
		"To":   {toNumber},
		"Body": {message},
	}
	resp, err := httpclient.DoHTTP(httpclient.Request{
		TimeoutSec: FeatureTestTimeoutSec,
		Method:     http.MethodPost,
		Body:       strings.NewReader(formParams.Encode()),
		RequestFunc: func(req *http.Request) error {
			req.SetBasicAuth(twi.AccountSID, twi.AuthToken)
			return nil
		},
	}, "https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", twi.AccountSID)
	if errResult := HTTPErrorToResult(resp, err); errResult != nil {
		return errResult
	}
	// The OK output is simply the length of number + message
	return &Result{Error: nil, Output: strconv.Itoa(len(toNumber) + len(message))}
}
