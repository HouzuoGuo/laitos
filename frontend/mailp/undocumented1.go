package mailp

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/httpclient"
	"net/http"
	"net/url"
	"strings"
)

const Undocumented1HTTPTimeoutSec = 30

// Intentionally undocumented he he he.
type Undocumented1 struct {
	URL   string `json:"URL"`
	Addr1 string `json:"Addr1"`
	Addr2 string `json:"Addr2"`
	ID1   string `json:"ID1"`
	ID2   string `json:"ID2"`
}

var TestUndocumented1 = Undocumented1{} // Details are set by init_mail_test.go

func (und *Undocumented1) IsConfigured() bool {
	return und.URL != "" && und.Addr1 != "" && und.Addr2 != "" && und.ID1 != "" && und.ID2 != ""
}

func (und *Undocumented1) SelfTest() error {
	if !und.IsConfigured() {
		return feature.ErrIncompleteConfig
	}
	resp, err := httpclient.DoHTTP(httpclient.Request{TimeoutSec: feature.TestTimeoutSec}, und.URL)
	// Only consider IO error and 404 response to be actual errors
	if err != nil {
		return err
	} else if resp.StatusCode == http.StatusNotFound {
		return errors.New("URL is not found")
	}
	return nil
}

func (und *Undocumented1) SendMessage(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("Undocumented1.SendMessage: message is empty")
	}

	resp, err := httpclient.DoHTTP(httpclient.Request{
		TimeoutSec: Undocumented1HTTPTimeoutSec,
		Method:     http.MethodPost,
		Body: strings.NewReader(url.Values{
			"MessageId":    {und.ID1},
			"Guid":         {und.ID2},
			"ReplyAddress": {und.Addr2},
			"ReplyMessage": {message},
		}.Encode()),
		RequestFunc: func(req *http.Request) error {
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/57.0.2987.133 Safari/537.36")
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
