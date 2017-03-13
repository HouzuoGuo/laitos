package api

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/common"
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
        <pre>%s</pre>
    </form>
</body>
</html>
` // Run Command page content

const CommandFormTimeoutSec = 25 // commands executed on the form

type HandleCommandForm struct {
}

func (_ *HandleCommandForm) MakeHandler(cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
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
				w.Write([]byte(fmt.Sprintf(HandleCommandFormPage, result.CombinedOutput)))
			}
		}
	}
	return fun, nil
}

func (_ *HandleCommandForm) GetRateLimitFactor() int {
	return 1
}
