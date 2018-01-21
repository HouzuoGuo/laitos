package handler

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/misc"
	"net/http"
)

const HandleNotificationSetupPage = `<!doctype html>
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
    <title>Notification setup</title>
</head>
<body>
    <form action="#" method="get">
        <p>
            Store new command (e.g. PIN.prefix params..): <input type="text" name="command" value="%s" />
		</p>
		<p>
            Or store an arbitrary text for later retrieval: <input type="text" name="text" value="%s" />
		</p>
		<p>
            <input type="submit" name="submit" value="Go"/>
			<input type="submit" name="submit" value="Clear all entered commands"/>
        </p>
    </form>
</body>
</html>
`

// HandleNotificationSetup is an HTML form for user to manipulate timer commands.
type HandleNotificationSetup struct {
	Timers map[string]*common.CommandTimer // Timers are mappings between arbitrary ID string and associated command timer.
	logger misc.Logger
}

func (setup *HandleNotificationSetup) Initialise(logger misc.Logger, _ *common.CommandProcessor) error {
	setup.logger = logger
	return nil
}

func (_ *HandleNotificationSetup) GetRateLimitFactor() int {
	return 5
}

func (_ *HandleNotificationSetup) SelfTest() error {
	return nil
}

func (setup *HandleNotificationSetup) Handle(w http.ResponseWriter, r *http.Request) {
	NoCache(w)
	if !WarnIfNoHTTPS(r, w) {
		return
	}
	newCommand := r.FormValue("command")
	textToStore := r.FormValue("text")
	submitAction := r.FormValue("submit")
	// TODO
	switch submitAction {
	case "Go":
		if newCommand != "" {

		} else if textToStore != "" {

		}
	case "Clear all entered commands":
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(fmt.Sprintf(HandleNotificationSetupPage, newCommand, textToStore)))
}

// HandleNotificationRetrieval responds with the latest timer command execution results in JSON.
type HandleNotificationRetrieval struct {
	Timers map[string]*common.CommandTimer // Timers are mappings between arbitrary ID string and associated command timer.
	logger misc.Logger
}

func (notif *HandleNotificationRetrieval) Initialise(logger misc.Logger, _ *common.CommandProcessor) error {
	notif.logger = logger
	return nil
}

func (_ *HandleNotificationRetrieval) GetRateLimitFactor() int {
	return 5
}

func (_ *HandleNotificationRetrieval) SelfTest() error {
	return nil
}

func (notif *HandleNotificationRetrieval) Handle(w http.ResponseWriter, r *http.Request) {
	NoCache(w)
	if !WarnIfNoHTTPS(r, w) { // really need this?
		return
	}
	// TODO
}
