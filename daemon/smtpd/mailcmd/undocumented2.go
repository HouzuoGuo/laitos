package mailcmd

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/toolbox"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const Undocumented2HTTPTimeoutSec = 30

// (TH) Intentionally undocumented he he he.
type Undocumented2 struct {
	URL            string `json:"URL"`
	MailAddrSuffix string `json:"MailAddrSuffix"`
	MsisDN         string `json:"MsisDN"`
	From           string `json:"From"`
}

var TestUndocumented2 = Undocumented2{} // Details are set by init_mail_test.go

func (und *Undocumented2) IsConfigured() bool {
	return und.URL != "" && und.MailAddrSuffix != "" && und.MsisDN != "" && und.From != ""
}

func (und *Undocumented2) SelfTest() error {
	if !und.IsConfigured() {
		return toolbox.ErrIncompleteConfig
	}
	resp, err := inet.DoHTTP(inet.HTTPRequest{TimeoutSec: toolbox.SelfTestTimeoutSec}, und.URL)
	// Only consider IO error and 404 response to be actual errors
	if err != nil {
		return err
	} else if resp.StatusCode == http.StatusNotFound {
		return errors.New("URL is not found")
	}
	return nil
}

func (und *Undocumented2) MayReplyTo(prop inet.BasicMail) bool {
	return und.IsConfigured() && und.MailAddrSuffix != "" && strings.HasSuffix(prop.ReplyAddress, und.MailAddrSuffix)
}

func (und *Undocumented2) SendMessage(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("Undocumented2.SendMessage: message is empty")
	}

	tlength := 160 - len(und.From) - len(message)
	if tlength < 0 {
		message = message[:len(message)-(-tlength)]
		tlength = 0
	}

	resp, err := inet.DoHTTP(inet.HTTPRequest{
		TimeoutSec: Undocumented1HTTPTimeoutSec,
		Method:     http.MethodPost,
		Body: strings.NewReader(url.Values{
			"msisdn":  {und.MsisDN},
			"from":    {und.From},
			"message": {message},
			"tlength": {strconv.Itoa(tlength)},
		}.Encode()),
		RequestFunc: func(req *http.Request) error {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
			return nil
		},
	}, und.URL)
	if err != nil {
		return fmt.Errorf("Undocumented2.SendMessage: HTTP failure - %v", err)
	} else if respErr := resp.Non2xxToError(); respErr != nil {
		return fmt.Errorf("Undocumented2.SendMessage: bad response - %v", respErr)
	}
	return nil
}
