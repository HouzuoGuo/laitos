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

// (IM) Intentionally undocumented he he he.
type Undocumented3 struct {
	URL            string `json:"URL"`
	MailAddrSuffix string `json:"MailAddrSuffix"`
	ToNumber       string `json:"ToNumber"`
	ReplyEmail     string `json:"ReplyEmail"`
}

var TestUndocumented3 = Undocumented3{} // Details are set by init_mail_test.go

func (und *Undocumented3) IsConfigured() bool {
	return und.URL != "" && und.MailAddrSuffix != "" && und.ToNumber != "" && und.ReplyEmail != ""
}

func (und *Undocumented3) SelfTest() error {
	if !und.IsConfigured() {
		return toolbox.ErrIncompleteConfig
	}
	_, err := inet.DoHTTP(inet.HTTPRequest{TimeoutSec: toolbox.SelfTestTimeoutSec}, und.URL)
	// Unlike the other two undocumented scenarios, 404 is not an indication of actual failure in this case.
	if err != nil {
		return err
	}
	return nil
}

func (und *Undocumented3) MayReplyTo(prop inet.BasicMail) bool {
	return und.IsConfigured() && und.MailAddrSuffix != "" && strings.HasSuffix(prop.ReplyAddress, und.MailAddrSuffix)
}

func (und *Undocumented3) SendMessage(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("Undocumented3.SendMessage: message is empty")
	}

	maxMessageLen := 134 - len(und.ReplyEmail)
	if len(message) > maxMessageLen {
		message = message[:maxMessageLen]
	}

	resp, err := inet.DoHTTP(inet.HTTPRequest{
		TimeoutSec: UndocumentedHTTPTimeoutSec,
		Method:     http.MethodPost,
		Body: strings.NewReader(url.Values{
			"to":          {und.ToNumber},
			"reply_email": {und.ReplyEmail},
			"message":     {message},
		}.Encode()),
		RequestFunc: func(req *http.Request) error {
			req.Header.Set("User-Agent", browser.GoodUserAgent)
			return nil
		},
	}, und.URL)
	if err != nil {
		return fmt.Errorf("Undocumented3.SendMessage: HTTP failure - %v", err)
	} else if respErr := resp.Non2xxToError(); respErr != nil {
		return fmt.Errorf("Undocumented3.SendMessage: bad response - %v", respErr)
	}
	return nil
}
