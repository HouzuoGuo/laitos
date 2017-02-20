package feature

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	TWILIO_MAKE_CALL = "c"
	TWILIO_SEND_SMS  = "s"
)

var RegexNumberAndMessage = regexp.MustCompile(`(\+[0-9]+)[^\w]+(.*)`)

type Twilio struct {
	PhoneNumber string // Twilio telephone number for outbound call and SMS
	AccountSID  string // Twilio account SID
	AuthSecret  string // Twilio authentication secret token

	TestPhoneNumber string // Set by init_test.go for running test case
}

var TestTwilio = Twilio{} // API credentials are set by init_test.go

func (twi *Twilio) IsConfigured() bool {
	return twi.PhoneNumber != "" && twi.AccountSID != "" && twi.AuthSecret != ""
}

func (twi *Twilio) SelfTest() error {
	if !twi.IsConfigured() {
		return ErrIncompleteConfig
	}
	// Make a test API call to validate credentials
	return twi.ValidateCredentials()
}

func (twi *Twilio) Initialise() error {
	return nil
}

func (twi *Twilio) TriggerPrefix() string {
	return ".c"
}

func (twi *Twilio) Execute(cmd Command) (ret *Result) {
	LogBeforeExecute(cmd)
	defer func() {
		LogAfterExecute(cmd, ret)
	}()
	if errResult := cmd.Trim(); errResult != nil {
		ret = errResult
		return
	}

	if strings.HasPrefix(cmd.Content, TWILIO_MAKE_CALL) {
		ret = twi.MakeCall(cmd)
	} else if strings.HasPrefix(cmd.Content, TWILIO_SEND_SMS) {
		ret = twi.SendSMS(cmd)
	} else {
		ret = &Result{Error: fmt.Errorf("Failed to find command prefix (either %s or %s)", TWILIO_MAKE_CALL, TWILIO_SEND_SMS)}
	}
	return
}

func (twi *Twilio) MakeCall(cmd Command) *Result {
	params := RegexNumberAndMessage.FindStringSubmatch(strings.TrimSpace(strings.TrimPrefix(cmd.Content, TWILIO_MAKE_CALL)))
	if len(params) < 3 {
		return &Result{Error: errors.New("Call parameters are missing")}
	}
	toNumber := params[1]
	message := params[2]

	formParams := url.Values{
		"From": []string{twi.PhoneNumber},
		"To":   []string{toNumber},
		"Url":  []string{"http://twimlets.com/message?Message=" + url.QueryEscape(fmt.Sprintf("%s repeat once again %s repeat once again %s over", message, message, message))}}

	status, resp, err := DoHTTP(cmd.TimeoutSec, "POST", "", strings.NewReader(formParams.Encode()), func(req *http.Request) error {
		req.SetBasicAuth(twi.AccountSID, twi.AuthSecret)
		return nil
	}, "https://api.twilio.com/2010-04-01/Accounts/%s/Calls.json", twi.AccountSID)
	if errResult := HTTPResponseError(status, resp, err); errResult != nil {
		return errResult
	}
	// The OK output is simply the length of number + message
	return &Result{Error: nil, Output: strconv.Itoa(len(toNumber) + len(message))}
}

func (twi *Twilio) SendSMS(cmd Command) *Result {
	params := RegexNumberAndMessage.FindStringSubmatch(strings.TrimSpace(strings.TrimPrefix(cmd.Content, TWILIO_MAKE_CALL)))
	if len(params) < 3 {
		return &Result{Error: errors.New("SMS parameters are missing")}
	}
	toNumber := params[1]
	message := params[2]

	formParams := url.Values{
		"From": []string{twi.PhoneNumber},
		"To":   []string{toNumber},
		"Body": []string{message}}

	status, resp, err := DoHTTP(cmd.TimeoutSec, "POST", "", strings.NewReader(formParams.Encode()), func(req *http.Request) error {
		req.SetBasicAuth(twi.AccountSID, twi.AuthSecret)
		return nil
	}, "https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", twi.AccountSID)
	if errResult := HTTPResponseError(status, resp, err); errResult != nil {
		return errResult
	}
	// The OK output is simply the length of number + message
	return &Result{Error: nil, Output: strconv.Itoa(len(toNumber) + len(message))}
}

// Validate my account credentials, return an error only if credentials are invalid.
func (twi *Twilio) ValidateCredentials() error {
	_, _, err := DoHTTP(HTTP_TEST_TIMEOUT_SEC, "GET", "", nil, func(req *http.Request) error {
		req.SetBasicAuth(twi.AccountSID, twi.AuthSecret)
		return nil
	}, "https://api.twilio.com/2010-04-01/Accounts/%s", twi.AccountSID)
	return err
}
