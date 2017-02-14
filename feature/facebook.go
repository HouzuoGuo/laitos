package feature

import (
	"errors"
	"log"
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

func (fb *Facebook) Initialise() error {
	log.Print("Facebook.Initialise: in progress")
	if !fb.IsConfigured() {
		return ErrIncompleteConfig
	}
	// Make a test API call to validate API access token
	if err := fb.ValidateAccessToken(); err != nil {
		return err
	}
	log.Print("Facebook.Initialise: successfully completed")
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
	_, resp, err := DoHTTP(30, "GET", "", nil, nil,
		"https://graph.facebook.com/2.8/me?access_token=%s", fb.AccessToken)
	if err != nil {
		return err
	} else if len(resp) < 10 {
		return errors.New("Response body seems too short: " + string(resp))
	}
	return nil
}
