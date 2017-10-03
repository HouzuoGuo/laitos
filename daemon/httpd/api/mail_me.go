package api

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"net/http"
)

const HandleMailMePage = `<!doctype html>
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
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
    <form action="#" method="post">
        <p><textarea name="msg" cols="30" rows="4"></textarea></p>
        <p><input type="submit" value="发出去"/></p>
        <p>%s</p>
    </form>
</body>
</html>
` // Mail Me page content

// Send Howard an email in a simple web form. The text on the page is deliberately written in Chinese.
type HandleMailMe struct {
	Recipients []string    `json:"Recipients"` // Recipients of these mail messages
	Mailer     inet.Mailer `json:"-"`
}

func (mm *HandleMailMe) MakeHandler(logger misc.Logger, _ *common.CommandProcessor) (http.HandlerFunc, error) {
	if mm.Recipients == nil || len(mm.Recipients) == 0 || !mm.Mailer.IsConfigured() {
		return nil, errors.New("HandleMailMe.MakeHandler: recipient list is empty or mailer is not configured")
	}
	fun := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		NoCache(w)
		if !WarnIfNoHTTPS(r, w) {
			return
		}
		if r.Method == http.MethodGet {
			// Render the page
			w.Write([]byte(fmt.Sprintf(HandleMailMePage, "")))
		} else if r.Method == http.MethodPost {
			// Retrieve message and deliver it
			if msg := r.FormValue("msg"); msg == "" {
				w.Write([]byte(fmt.Sprintf(HandleMailMePage, "")))
			} else {
				prompt := "出问题了，发不出去。"
				if err := mm.Mailer.Send(inet.OutgoingMailSubjectKeyword+"-mailme", msg, mm.Recipients...); err == nil {
					prompt = "发出去了。可以接着写。"
				} else {
					logger.Warningf("HandleMailMe", r.RemoteAddr, err, "failed to deliver mail")
				}
				w.Write([]byte(fmt.Sprintf(HandleMailMePage, prompt)))
			}
		}
	}
	return fun, nil
}

func (mm *HandleMailMe) GetRateLimitFactor() int {
	return 1
}
