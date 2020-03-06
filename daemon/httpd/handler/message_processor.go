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

// HandleReportsRetrieval reads the latest reports from store&forward message processor.
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

	// The default maximum number of reports to retrieve is 1000
	limitStr := r.FormValue("n")
	limitNum, _ := strconv.Atoi(limitStr)
	if limitNum < 1 {
		limitNum = 1000
	}
	host := r.FormValue("host")
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
