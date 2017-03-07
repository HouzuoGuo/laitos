package feature

import (
	"errors"
	"github.com/HouzuoGuo/websh/httpclient"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// An undocumented way to send message to myself.
type Undocumented1 struct {
	URL   string `json:"URL"`
	Addr1 string `json:"Addr1"`
	Addr2 string `json:"Addr2"`
	ID1   string `json:"ID1"`
	ID2   string `json:"ID2"`
}

var TestUndocumented1 = Undocumented1{} // Details are set by init_test.go

func (und *Undocumented1) IsConfigured() bool {
	return und.URL != "" && und.Addr1 != "" && und.Addr2 != "" && und.ID1 != "" && und.ID2 != ""
}

func (und *Undocumented1) SelfTest() error {
	if !und.IsConfigured() {
		return ErrIncompleteConfig
	}
	resp, err := httpclient.DoHTTP(httpclient.Request{TimeoutSec: FeatureTestTimeoutSec}, und.URL)
	// Only consider IO error and 404 response to be actual errors
	if err != nil {
		return err
	} else if resp.StatusCode == http.StatusNotFound {
		return errors.New("URL responsed with status 404")
	}
	return nil
}

func (und *Undocumented1) Initialise() error {
	return nil
}

func (und *Undocumented1) Trigger() Trigger {
	return "NOT-TO-BE-TRIGGERED-MANUALLY-UNDOCUMENTED1"
}

func (und *Undocumented1) Execute(cmd Command) (ret *Result) {
	if errResult := cmd.Trim(); errResult != nil {
		ret = errResult
		return
	}

	resp, err := httpclient.DoHTTP(httpclient.Request{
		TimeoutSec: cmd.TimeoutSec,
		Method:     http.MethodPost,
		Body: strings.NewReader(url.Values{
			"MessageId":    {und.ID1},
			"Guid":         {und.ID2},
			"ReplyAddress": {und.Addr2},
			"ReplyMessage": {cmd.Content},
		}.Encode()),
		RequestFunc: func(req *http.Request) error {
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.100 Safari/537.36")
			return nil
		},
	}, und.URL)
	if errResult := HTTPErrorToResult(resp, err); errResult != nil {
		ret = errResult
	} else {
		// The OK output is simply the length message
		ret = &Result{Error: nil, Output: strconv.Itoa(len(cmd.Content))}
	}
	return
}
