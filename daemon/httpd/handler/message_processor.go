package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

/*
HandleReportsRetrieval works as a frontend to the store&forward message processor, allowing visitors to view historical reports and
assign an app command for a subject to retireve in its next report.
*/
type HandleReportsRetrieval struct {
	cmdProc *toolbox.CommandProcessor
}

func (hand *HandleReportsRetrieval) Initialise(_ lalog.Logger, cmdProc *toolbox.CommandProcessor) error {
	if cmdProc == nil {
		return errors.New("HandleReportsRetrieval.Initialise: command processor must not be nil")
	}
	if errs := cmdProc.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("HandleReportsRetrieval.Initialise: %+v", errs)
	}
	hand.cmdProc = cmdProc
	return nil
}

func (hand *HandleReportsRetrieval) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	NoCache(w)

	// endpoint/..?tohost=abc&cmd=xxxxxx
	toHost := r.FormValue("tohost")
	upcomingAppCmd := r.FormValue("cmd")
	if toHost != "" {
		hand.cmdProc.Features.MessageProcessor.SetUpcomingSubjectCommand(toHost, upcomingAppCmd)
		_, _ = w.Write([]byte(fmt.Sprintf("OK, the next reply made in response to %s's report will carry an app command %d characters long.", toHost, len(upcomingAppCmd))))
		return
	}

	// endpoint/...?n=123&host=abc
	host := r.FormValue("host")
	limitStr := r.FormValue("n")
	limitNum, _ := strconv.Atoi(limitStr)
	if limitNum < 1 {
		// The default maximum number of reports to retrieve is 1000
		limitNum = 1000
	}
	jsonWriter := json.NewEncoder(w)
	jsonWriter.SetIndent("", "  ")
	if host == "" {
		// Get the latest reports across all hosts
		w.WriteHeader(200)
		if err := jsonWriter.Encode(hand.cmdProc.Features.MessageProcessor.GetLatestReports(limitNum)); err != nil {
			lalog.DefaultLogger.Warning("HandleReportsRetrieval", r.Host, err, "failed to serialise JSON response")
		}
	} else {
		// Get the latest reports from a particular host
		w.WriteHeader(200)
		if err := jsonWriter.Encode(hand.cmdProc.Features.MessageProcessor.GetLatestReportsFromSubject(host, limitNum)); err != nil {
			lalog.DefaultLogger.Warning("HandleReportsRetrieval", r.Host, err, "failed to serialise JSON response")
		}
	}
}

func (hand *HandleReportsRetrieval) GetRateLimitFactor() int {
	return 1
}

func (_ *HandleReportsRetrieval) SelfTest() error {
	return nil
}

// HandleAppCommand executes app command from the incoming request.
type HandleAppCommand struct {
	cmdProc *toolbox.CommandProcessor
}

func (hand *HandleAppCommand) Initialise(_ lalog.Logger, cmdProc *toolbox.CommandProcessor) error {
	if cmdProc == nil {
		return errors.New("HandleAppCommand.Initialise: command processor must not be nil")
	}
	if errs := cmdProc.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("HandleAppCommand.Initialise: %+v", errs)
	}
	hand.cmdProc = cmdProc
	return nil
}

func (hand *HandleAppCommand) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	NoCache(w)
	result := hand.cmdProc.Process(toolbox.Command{
		DaemonName: "httpd",
		ClientID:   GetRealClientIP(r),
		Content:    r.FormValue("cmd"),
		TimeoutSec: HTTPClienAppCommandTimeout,
	}, true)
	_, _ = w.Write([]byte(result.CombinedOutput))
}

func (hand *HandleAppCommand) GetRateLimitFactor() int {
	return 6
}

func (_ *HandleAppCommand) SelfTest() error {
	return nil
}
