package handler

import (
	"encoding/json"
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
			For pre-configured channel <input type="text" name="channel" value="%s" />:
		</p>
		<ul>
			<li>Store new command (e.g. PIN.prefix params..): <input type="text" name="command" value="%s" /></li>
			<li>Or store an arbitrary text for later retrieval: <input type="text" name="text" value="%s" /></li>
		</ul>
		<p>
            <input type="submit" name="submit" value="Go"/>
			<input type="submit" name="submit" value="Clear all entered commands"/>
        </p>
		<pre>%s</pre>
    </form>
</body>
</html>
`

/*
HandleNotification is an HTML form for user to manipulate timer commands.
The timers inside should be shared with a HandleNotificationRetrieval HTTP handler.
*/
type HandleNotification struct {
	Timers map[string]*common.CommandTimer `json:"Timers"` // Timers are mappings between arbitrary ID string and associated command timer.
	logger misc.Logger
}

func (notif *HandleNotification) Initialise(logger misc.Logger, cmdProc *common.CommandProcessor) error {
	notif.logger = logger
	if notif.Timers == nil || len(notif.Timers) == 0 {
		return fmt.Errorf("HandleNotification: there are no timers")
	}
	for _, timer := range notif.Timers {
		timer.CommandProcessor = cmdProc
		if err := timer.Initialise(); err != nil {
			return err
		}
		go timer.Start()
		// Because handlers do not have tear-down function, there is no way to stop them. Consider fixing this in the future?
	}
	return nil
}

func (_ *HandleNotification) GetRateLimitFactor() int {
	return 5
}

func (_ *HandleNotification) SelfTest() error {
	return nil
}

func (notif *HandleNotification) Handle(w http.ResponseWriter, r *http.Request) {
	NoCache(w)
	if !WarnIfNoHTTPS(r, w) {
		return
	}
	if retrieveFromChannel := r.FormValue("retrieve"); retrieveFromChannel == "" {
		// Serve HTML page for setting up notifications
		channel := r.FormValue("channel")
		newCommand := r.FormValue("command")
		textToStore := r.FormValue("text")
		submitAction := r.FormValue("submit")
		var conclusion string
		switch submitAction {
		case "Go":
			if channel == "" {
				conclusion = "Please enter pre-configured channel ID."
			} else if newCommand != "" {
				// Store a new command
				timer, exists := notif.Timers[channel]
				if exists {
					timer.AddTransientCommand(newCommand)
					conclusion = "Successfully stored new command: " + newCommand
				} else {
					conclusion = "Cannot find channel ID: " + channel
				}
			} else if textToStore != "" {
				// Store arbitrary text message
				timer, exists := notif.Timers[channel]
				if exists {
					timer.AddArbitraryTextToResult(textToStore)
					conclusion = "Successfully stored text message: " + textToStore
				} else {
					conclusion = "Cannot find channel ID: " + channel
				}
			} else {
				conclusion = "Please enter a new command or text message to store."
			}
		case "Clear all entered commands":
			timer, exists := notif.Timers[channel]
			if exists {
				timer.ClearTransientCommands()
				conclusion = "All newly stored commands have been cleared for: " + channel
			} else {
				conclusion = "Cannot find channel ID: " + channel
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(fmt.Sprintf(HandleNotificationSetupPage, channel, newCommand, textToStore, conclusion)))
	} else {
		// Retrieve results in JSON format
		timer, exists := notif.Timers[retrieveFromChannel]
		if exists {
			resp, err := json.Marshal(timer.GetResults())
			if err == nil {
				w.Write(resp)
			} else {
				http.Error(w, "JSON serialisation failure: "+err.Error(), http.StatusInternalServerError)
			}
		} else {
			http.Error(w, "Cannot find channel ID: "+retrieveFromChannel, http.StatusNotFound)
		}
	}
}
