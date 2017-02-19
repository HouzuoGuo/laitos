package feature

import (
	"net/url"
	"strconv"
	"strings"
)

type Facebook struct {
	AccessToken string
}

var TestFacebook = Facebook{} // API access token is set by init_test.go

func (fb *Facebook) IsConfigured() bool {
	return fb.AccessToken != ""
}

func (fb *Facebook) SelfTest() error {
	if !fb.IsConfigured() {
		return ErrIncompleteConfig
	}
	return fb.ValidateAccessToken()
}

func (fb *Facebook) Initialise() error {
	return nil
}

func (fb *Facebook) TriggerPrefix() string {
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
		"https://graph.facebook.com/v2.8/me/feed?access_token=%s", fb.AccessToken)
	if errResult := HTTPResponseError(status, resp, err); errResult != nil {
		return errResult
	}
	// The OK output is simply the length of posted message
	return &Result{Error: nil, Output: strconv.Itoa(len(cmd.Content))}
}

func (fb *Facebook) ValidateAccessToken() error {
	_, _, err := DoHTTP(30, "GET", "", nil, nil,
		"https://graph.facebook.com/2.8/me?access_token=%s", fb.AccessToken)
	return err
}
