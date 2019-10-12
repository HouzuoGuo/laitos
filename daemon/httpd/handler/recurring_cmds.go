package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/lalog"
)

const HandleRecurringCommandsSetupPage = `<html>
<head>
    <title>Recurring commands setup</title>
</head>
<body>
    <form action="%s" method="post">
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
HandleRecurringCommands is an HTML form for user to manipulate recurring commands, such as adding/clearing transient
commands and pushing text message directly into result.
*/
type HandleRecurringCommands struct {
	RecurringCommands map[string]*common.RecurringCommands `json:"RecurringCommands"` // are mappings between arbitrary ID string and associated command timer.
	logger            lalog.Logger
}

func (notif *HandleRecurringCommands) Initialise(logger lalog.Logger, cmdProc *common.CommandProcessor) error {
	notif.logger = logger
	if notif.RecurringCommands == nil || len(notif.RecurringCommands) == 0 {
		return fmt.Errorf("HandleRecurringCommands: there must be at least one recurring command channel in configuration")
	}
	for _, timer := range notif.RecurringCommands {
		timer.CommandProcessor = cmdProc
		if err := timer.Initialise(); err != nil {
			return err
		}
		go timer.Start()
		// Because handlers do not have tear-down function, there is no way to stop them. Consider fixing this in the future?
	}
	return nil
}

func (_ *HandleRecurringCommands) GetRateLimitFactor() int {
	return 4
}

func (_ *HandleRecurringCommands) SelfTest() error {
	return nil
}

func (notif *HandleRecurringCommands) Handle(w http.ResponseWriter, r *http.Request) {
	NoCache(w)
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
				timer, exists := notif.RecurringCommands[channel]
				if exists {
					timer.AddTransientCommand(newCommand)
					conclusion = "Successfully stored new command: " + newCommand
				} else {
					conclusion = "Cannot find channel ID: " + channel
				}
			} else if textToStore != "" {
				// Store arbitrary text message
				timer, exists := notif.RecurringCommands[channel]
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
			timer, exists := notif.RecurringCommands[channel]
			if exists {
				timer.ClearTransientCommands()
				conclusion = "All newly stored commands have been cleared for: " + channel
			} else {
				conclusion = "Cannot find channel ID: " + channel
			}

		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fmt.Sprintf(HandleRecurringCommandsSetupPage, r.RequestURI, channel, newCommand, textToStore, conclusion)))
	} else {

		// Retrieve results in JSON format
		timer, exists := notif.RecurringCommands[retrieveFromChannel]
		if exists {
			resp, err := json.Marshal(timer.GetResults())
			if err == nil {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(resp)
			} else {
				http.Error(w, "JSON serialisation failure: "+err.Error(), http.StatusInternalServerError)
			}
		} else {
			http.Error(w, "Cannot find channel ID: "+retrieveFromChannel, http.StatusNotFound)
		}
	}
}
