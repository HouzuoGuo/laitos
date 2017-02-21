package feature

import (
	"net/url"
	"strconv"
	"strings"
)

type Facebook struct {
	// Facebook user access token has a validity of 60 days, so remember to get a new one manually every 60 days!
	UserAccessToken string `json:"UserAccessToken"` // Facebook API user access token (not to be confused with App ID or App Secret)
}

var TestFacebook = Facebook{} // API access token is set by init_test.go

func (fb *Facebook) IsConfigured() bool {
	return fb.UserAccessToken != ""
}

func (fb *Facebook) SelfTest() error {
	if !fb.IsConfigured() {
		return ErrIncompleteConfig
	}
	// Validate access token via a simple API call
	status, resp, err := DoHTTP(HTTP_TEST_TIMEOUT_SEC, "GET", "", nil, nil,
		"https://graph.facebook.com/v2.8/me/feed?access_token=%s", fb.UserAccessToken)
	return HTTPResponseError(status, resp, err)
}

func (fb *Facebook) Initialise() error {
	return nil
}

func (fb *Facebook) Trigger() Trigger {
	return ".f"
}

func (fb *Facebook) Execute(cmd Command) (ret *Result) {
	LogBeforeExecute(cmd)
	defer func() {
		LogAfterExecute(cmd, ret)
	}()
	if errResult := cmd.Trim(); errResult != nil {
		ret = errResult
		return
	}

	status, resp, err := DoHTTP(cmd.TimeoutSec, "POST", "", strings.NewReader(url.Values{"message": []string{cmd.Content}}.Encode()), nil,
		"https://graph.facebook.com/v2.8/me/feed?access_token=%s", fb.UserAccessToken)
	if errResult := HTTPResponseErrorResult(status, resp, err); errResult != nil {
		return errResult
	}
	// The OK output is simply the length of posted message
	return &Result{Error: nil, Output: strconv.Itoa(len(cmd.Content))}
}
