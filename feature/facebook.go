package feature

import (
	"github.com/HouzuoGuo/websh/httpclient"
	"net/http"
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
	resp, err := httpclient.DoHTTP(httpclient.Request{TimeoutSec: HTTPTestTimeoutSec}, "https://graph.facebook.com/v2.8/me/feed?access_token=%s", fb.UserAccessToken)
	if err != nil {
		return err
	}
	return resp.Non2xxToError()
}

func (fb *Facebook) Initialise() error {
	return nil
}

func (fb *Facebook) Trigger() Trigger {
	return ".f"
}

func (fb *Facebook) Execute(cmd Command) (ret *Result) {
	if errResult := cmd.Trim(); errResult != nil {
		ret = errResult
		return
	}

	resp, err := httpclient.DoHTTP(httpclient.Request{
		TimeoutSec: HTTPTestTimeoutSec,
		Method:     http.MethodPost,
		Body:       strings.NewReader(url.Values{"message": []string{cmd.Content}}.Encode()),
	}, "https://graph.facebook.com/v2.8/me/feed?access_token=%s", fb.UserAccessToken)

	if errResult := HTTPErrorToResult(resp, err); errResult == nil {
		// The OK output is simply the length of posted message
		ret = &Result{Error: nil, Output: strconv.Itoa(len(cmd.Content))}
	} else {
		ret = errResult
	}
	return
}
