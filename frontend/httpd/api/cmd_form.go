package api

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/global"
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
` // Run Command page content

const CommandFormTimeoutSec = 110 // Form commands may enjoy a less constrained timeout

type HandleCommandForm struct {
}

func (_ *HandleCommandForm) MakeHandler(logger global.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	fun := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "must-revalidate")
		if r.Method == http.MethodGet {
			w.Write([]byte(fmt.Sprintf(HandleCommandFormPage, "")))
		} else if r.Method == http.MethodPost {
			if cmd := r.FormValue("cmd"); cmd == "" {
				w.Write([]byte(fmt.Sprintf(HandleCommandFormPage, "")))
			} else {
				result := cmdProc.Process(feature.Command{
					Content:    cmd,
					TimeoutSec: CommandFormTimeoutSec,
				})
				w.Write([]byte(fmt.Sprintf(HandleCommandFormPage, html.EscapeString(result.CombinedOutput))))
			}
		}
	}
	return fun, nil
}

func (_ *HandleCommandForm) GetRateLimitFactor() int {
	return 1
}
