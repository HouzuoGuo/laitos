package handler

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/toolbox"
	"html"
	"net/http"
)

const HandleCommandFormPage = `<!doctype html>
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
    <title>Command Form</title>
</head>
<body>
    <form action="#" method="post">
        <p><input type="password" name="cmd" /><input type="submit" value="Exec"/></p>
        <textarea rows="12" cols="80">%s</textarea>
    </form>
</body>
</html>
` // HandleCommandFormPage is the command form's HTML content

// CommandFormTimeoutSec is the default command timeout in seconds. It should be less than the IO timeout of HTTP server.
const CommandFormTimeoutSec = 110

// Run feature commands in a simple web form.
type HandleCommandForm struct {
	cmdProc *common.CommandProcessor
}

func (form *HandleCommandForm) Initialise(_ misc.Logger, cmdProc *common.CommandProcessor) error {
	if cmdProc == nil {
		return errors.New("HandleCommandForm.Initialise: command processor must not be nil")
	}
	form.cmdProc = cmdProc
	return nil
}

func (form *HandleCommandForm) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	NoCache(w)
	if !WarnIfNoHTTPS(r, w) {
		return
	}
	if r.Method == http.MethodGet {
		w.Write([]byte(fmt.Sprintf(HandleCommandFormPage, "")))
	} else if r.Method == http.MethodPost {
		if cmd := r.FormValue("cmd"); cmd == "" {
			w.Write([]byte(fmt.Sprintf(HandleCommandFormPage, "")))
		} else {
			result := form.cmdProc.Process(toolbox.Command{
				Content:    cmd,
				TimeoutSec: CommandFormTimeoutSec,
			})
			w.Write([]byte(fmt.Sprintf(HandleCommandFormPage, html.EscapeString(result.CombinedOutput))))
		}
	}
}

func (_ *HandleCommandForm) GetRateLimitFactor() int {
	return 1
}

func (_ *HandleCommandForm) SelfTest() error {
	return nil
}
