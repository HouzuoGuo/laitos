package handler

import (
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/HouzuoGuo/laitos/daemon/httpd/middleware"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const HandleCommandFormPage = `<html>
<head>
    <title>Command Form</title>
</head>
<body>
    <form action="%s" method="post">
        <p><input type="password" name="cmd" /><input type="submit" value="Exec"/></p>
        <pre>%s</pre>
    </form>
</body>
</html>
` // HandleCommandFormPage is the command form's HTML content

// HTTPClienAppCommandTimeout is the timeout of app command execution in seconds shared by all capable HTTP endpoints.
const HTTPClienAppCommandTimeout = 59

// Run feature commands in a simple web form.
type HandleCommandForm struct {
	cmdProc                    *toolbox.CommandProcessor
	stripURLPrefixFromResponse string
}

func (form *HandleCommandForm) Initialise(_ lalog.Logger, cmdProc *toolbox.CommandProcessor, stripURLPrefixFromResponse string) error {
	if cmdProc == nil {
		return errors.New("HandleCommandForm.Initialise: command processor must not be nil")
	}
	if errs := cmdProc.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("HandleCommandForm.Initialise: %+v", errs)
	}
	form.cmdProc = cmdProc
	form.stripURLPrefixFromResponse = stripURLPrefixFromResponse
	return nil
}

func (form *HandleCommandForm) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	NoCache(w)
	if r.Method == http.MethodGet {
		_, _ = w.Write([]byte(fmt.Sprintf(HandleCommandFormPage, strings.TrimPrefix(r.RequestURI, form.stripURLPrefixFromResponse), "")))
	} else if r.Method == http.MethodPost {
		if cmd := r.FormValue("cmd"); cmd == "" {
			_, _ = w.Write([]byte(fmt.Sprintf(HandleCommandFormPage, strings.TrimPrefix(r.RequestURI, form.stripURLPrefixFromResponse), "")))
		} else {
			result := form.cmdProc.Process(r.Context(), toolbox.Command{
				DaemonName: "httpd",
				ClientTag:  middleware.GetRealClientIP(r),
				Content:    cmd,
				TimeoutSec: HTTPClienAppCommandTimeout,
			}, true)
			_, _ = w.Write([]byte(fmt.Sprintf(HandleCommandFormPage, strings.TrimPrefix(r.RequestURI, form.stripURLPrefixFromResponse), html.EscapeString(result.CombinedOutput))))
		}
	}
}

func (_ *HandleCommandForm) GetRateLimitFactor() int {
	return 1
}

func (_ *HandleCommandForm) SelfTest() error {
	return nil
}
