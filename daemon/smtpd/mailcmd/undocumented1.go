package mailcmd

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/browser"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/toolbox"
	"net/http"
	"net/url"
	"strings"
)

const UndocumentedHTTPTimeoutSec = 30

// (GA - DE - IR) Intentionally undocumented he he he.
type Undocumented1 struct {
	URL            string `json:"URL"`
	MailAddrSuffix string `json:"MailAddrSuffix"`
	ReplyAddress   string `json:"ReplyAddress"`
	MessageID      string `json:"MessageID"`
	GUID           string `json:"GUID"`
}

var TestUndocumented1 = Undocumented1{} // Details are set by init_mail_test.go

func (und *Undocumented1) IsConfigured() bool {
	return und.URL != "" && und.MailAddrSuffix != "" && und.ReplyAddress != "" && und.MessageID != "" && und.GUID != ""
}

func (und *Undocumented1) SelfTest() error {
	if !und.IsConfigured() {
		return toolbox.ErrIncompleteConfig
	}
	resp, err := inet.DoHTTP(inet.HTTPRequest{TimeoutSec: toolbox.SelfTestTimeoutSec}, und.URL)
	// Only consider IO error and 404 response to be actual errors. Other status codes most likely result from incomplete request.
	if err != nil {
		return err
	} else if resp.StatusCode == http.StatusNotFound {
		return errors.New("URL is not found")
	}
	return nil
}

func (und *Undocumented1) MayReplyTo(prop inet.BasicMail) bool {
	return und.IsConfigured() && und.MailAddrSuffix != "" && strings.HasSuffix(prop.ReplyAddress, und.MailAddrSuffix)
}

func (und *Undocumented1) SendMessage(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("Undocumented1.SendMessage: message is empty")
	}

	if len(message) > 160 {
		message = message[:160]
	}

	resp, err := inet.DoHTTP(inet.HTTPRequest{
		TimeoutSec: UndocumentedHTTPTimeoutSec,
		Method:     http.MethodPost,
		Body: strings.NewReader(url.Values{
			"MessageId":    {und.MessageID},
			"Guid":         {und.GUID},
			"ReplyAddress": {und.ReplyAddress},
			"ReplyMessage": {message},
		}.Encode()),
		RequestFunc: func(req *http.Request) error {
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
			req.Header.Set("User-Agent", browser.GoodUserAgent)
			return nil
		},
	}, und.URL)
	if err != nil {
		return fmt.Errorf("Undocumented1.SendMessage: HTTP failure - %v", err)
	} else if respErr := resp.Non2xxToError(); respErr != nil {
		return fmt.Errorf("Undocumented1.SendMessage: bad response - %v", respErr)
	}
	return nil
}
