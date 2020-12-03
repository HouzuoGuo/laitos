package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/platform/procexp"
	"github.com/HouzuoGuo/laitos/toolbox"
)

/*
HandleProcessExplorer is an HTTP handler that responds with process IDs that are running on the system, and when given a PID as query
parameter, the handler inspects the process for its current status and activities for the response.
*/
type HandleProcessExplorer struct {
	logger                     lalog.Logger
	stripURLPrefixFromResponse string
}

func (explorer *HandleProcessExplorer) Initialise(logger lalog.Logger, _ *toolbox.CommandProcessor, stripURLPrefixFromResponse string) error {
	explorer.logger = logger
	explorer.stripURLPrefixFromResponse = stripURLPrefixFromResponse
	return nil
}

func (_ *HandleProcessExplorer) GetRateLimitFactor() int {
	return 1
}

func (explorer *HandleProcessExplorer) SelfTest() error {
	return nil
}

func (explorer *HandleProcessExplorer) Handle(w http.ResponseWriter, r *http.Request) {
	NoCache(w)
	pidStr := r.FormValue("pid")
	respEncoder := json.NewEncoder(w)
	respEncoder.SetIndent("", "  ")
	if pidStr == "" {
		// Respond with a JSON array of PIDs available for choosing
		w.Header().Set("Content-Type", "application/json")
		_ = respEncoder.Encode(procexp.GetProcIDs())
	} else {
		// Respond with the latest process status
		pid, _ := strconv.Atoi(pidStr)
		// By contract, the function will retrieve the own process' status if the input PID is 0.
		status, err := procexp.GetProcStatus(pid)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read process status - %v", err), http.StatusInternalServerError)
			return
		}
		_ = respEncoder.Encode(status)
	}
}
