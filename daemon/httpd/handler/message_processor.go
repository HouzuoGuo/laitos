package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/HouzuoGuo/laitos/daemon/httpd/middleware"
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

func (hand *HandleReportsRetrieval) Initialise(_ lalog.Logger, cmdProc *toolbox.CommandProcessor, _ string) error {
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
	NoCache(w)

	host := r.FormValue("host")
	outgoingAppCmd := r.FormValue("cmd")
	clearOutgoingCmd := r.FormValue("clear")

	// Save / update / clear commands directed at a subject
	if outgoingAppCmd != "" || clearOutgoingCmd != "" {
		w.Header().Set("Content-Type", "text/plain")
		if clearOutgoingCmd == "" {
			// Store/update an outgoing command directed at a subject identified by its host name (/endpoint?tohost=abc&cmd=xxxxx)
			if outgoingAppCmd != "" {
				hand.cmdProc.Features.MessageProcessor.SetOutgoingCommand(host, outgoingAppCmd)
				_, _ = w.Write([]byte(fmt.Sprintf("The next reply made in response to %s's report will carry an app command %d characters long.\r\n", host, len(outgoingAppCmd))))
			}
		} else {
			// Clear an outgoing command directed at a subject identified by its host name (/endpoint?tohost=abc&clear=x)
			hand.cmdProc.Features.MessageProcessor.SetOutgoingCommand(host, "")
			_, _ = w.Write([]byte(fmt.Sprintf("Cleared outgoing command for host %s.\r\n", host)))
		}
		_, _ = w.Write([]byte("All outgoing commands:\r\n"))
		for host, cmd := range hand.cmdProc.Features.MessageProcessor.GetAllOutgoingCommands() {
			_, _ = w.Write([]byte(fmt.Sprintf("%s: %v\r\n", host, cmd)))
		}
		return
	}

	// Browse subjects and retrieve their reports
	w.Header().Set("Content-Type", "application/json")
	jsonWriter := json.NewEncoder(w)
	jsonWriter.SetIndent("", "  ")
	limitStr := r.FormValue("n")
	limitNum, _ := strconv.Atoi(limitStr)
	if limitNum < 1 {
		// Take a look at all subjects and count how many of their reports are currently stored in memory
		hand.cmdProc.Features.MessageProcessor.GetSubjectReportCount()
		w.WriteHeader(http.StatusOK)
		if err := jsonWriter.Encode(hand.cmdProc.Features.MessageProcessor.GetSubjectReportCount()); err != nil {
			lalog.DefaultLogger.Warning("HandleReportsRetrieval", r.Host, err, "failed to serialise JSON response")
		}
	} else {
		if host == "" {
			// Get the latest reports across all hosts
			w.WriteHeader(http.StatusOK)
			if err := jsonWriter.Encode(hand.cmdProc.Features.MessageProcessor.GetLatestReports(limitNum)); err != nil {
				lalog.DefaultLogger.Warning("HandleReportsRetrieval", r.Host, err, "failed to serialise JSON response")
			}
		} else {
			// Get the latest reports from a particular host
			w.WriteHeader(http.StatusOK)
			if err := jsonWriter.Encode(hand.cmdProc.Features.MessageProcessor.GetLatestReportsFromSubject(host, limitNum)); err != nil {
				lalog.DefaultLogger.Warning("HandleReportsRetrieval", r.Host, err, "failed to serialise JSON response")
			}
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

func (hand *HandleAppCommand) Initialise(_ lalog.Logger, cmdProc *toolbox.CommandProcessor, _ string) error {
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
	cmd := r.FormValue("cmd")
	if cmd == "" {
		// Ignore request that does not carry an app command
		w.WriteHeader(http.StatusOK)
		return
	}
	result := hand.cmdProc.Process(r.Context(), toolbox.Command{
		DaemonName: "httpd",
		ClientTag:  middleware.GetRealClientIP(r),
		Content:    cmd,
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
