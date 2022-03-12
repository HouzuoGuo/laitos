package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const HandleMailMePage = `<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
    <title>给厚佐写信</title>
    <style>
    	textarea {
    		font-size: 20px;
    		font-weight: bold;
    	}
    	p {
    		font-size: 20px;
    		font-weight: bold;
    	}
    	input {
    		font-size: 20px;
    		font-weight: bold;
    	}
    </style>
</head>
<body>
    <form action="%s" method="post">
        <p><textarea name="msg" cols="30" rows="4"></textarea></p>
        <p><input type="submit" value="发出去"/></p>
        <p>%s</p>
    </form>
</body>
</html>
` // Mail Me page content

// Send Howard an email in a simple web form. The text on the page is deliberately written in Chinese.
type HandleMailMe struct {
	Recipients []string        `json:"Recipients"` // Recipients of these mail messages
	MailClient inet.MailClient `json:"-"`

	stripURLPrefixFromResponse string
	logger                     lalog.Logger
}

func (mm *HandleMailMe) Initialise(logger lalog.Logger, _ *toolbox.CommandProcessor, stripURLPrefixFromResponse string) error {
	mm.logger = logger
	if mm.Recipients == nil || len(mm.Recipients) == 0 || !mm.MailClient.IsConfigured() {
		return errors.New("HandleMailMe.Initialise: recipient list is empty or mailer is not configured")
	}
	mm.stripURLPrefixFromResponse = stripURLPrefixFromResponse
	return nil
}

func (mm *HandleMailMe) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	NoCache(w)
	if r.Method == http.MethodGet {
		// Render the page
		_, _ = w.Write([]byte(fmt.Sprintf(HandleMailMePage, strings.TrimPrefix(r.RequestURI, mm.stripURLPrefixFromResponse), "")))
	} else if r.Method == http.MethodPost {
		// Retrieve message and deliver it
		if msg := r.FormValue("msg"); msg == "" {
			_, _ = w.Write([]byte(fmt.Sprintf(HandleMailMePage, strings.TrimPrefix(r.RequestURI, mm.stripURLPrefixFromResponse), "")))
		} else {
			prompt := "出问题了，发不出去。"
			if err := mm.MailClient.Send(inet.OutgoingMailSubjectKeyword+"-mailme", msg, mm.Recipients...); err == nil {
				prompt = "发出去了。可以接着写。"
			} else {
				mm.logger.Warning("HandleMailMe", r.RemoteAddr, err, "failed to deliver mail")
			}
			_, _ = w.Write([]byte(fmt.Sprintf(HandleMailMePage, strings.TrimPrefix(r.RequestURI, mm.stripURLPrefixFromResponse), prompt)))
		}
	}
}

func (mm *HandleMailMe) GetRateLimitFactor() int {
	return 1
}

func (mm *HandleMailMe) SelfTest() error {
	if err := mm.MailClient.SelfTest(); err != nil {
		return fmt.Errorf("HandleMailMe encountered a mail client error - %v", err)
	}
	return nil
}
