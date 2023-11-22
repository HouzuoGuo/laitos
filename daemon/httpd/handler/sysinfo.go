package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
	"github.com/HouzuoGuo/laitos/toolbox"
)

type systemInfo struct {
	Status platform.ProgramStatusSummary `json:"Status"`
	Stats  misc.ProgramStats             `json:"Stats"`
}

// HandleSystemInfo inspects system and application environment and returns them in text report.
type HandleSystemInfo struct {
	FeaturesToCheck    *toolbox.FeatureSet    `json:"-"` // Health check subject - features and their API keys
	CheckMailCmdRunner *mailcmd.CommandRunner `json:"-"` // Health check subject - mail processor and its mailer
	logger             *lalog.Logger
}

func (info *HandleSystemInfo) Initialise(logger *lalog.Logger, _ *toolbox.CommandProcessor, _ string) error {
	info.logger = logger
	return nil
}

func (info *HandleSystemInfo) handlePlainText(w http.ResponseWriter, r *http.Request) {
	// The routine is quite similar to maintenance daemon
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	NoCache(w)
	var result bytes.Buffer
	// Latest runtime info
	summary := platform.GetProgramStatusSummary(true)
	result.WriteString(summary.String())
	// Latest stats
	result.WriteString("\nDaemon stats - low/avg/high/total seconds and (count):\n")
	result.WriteString(misc.GetLatestStats())
	// Warnings, logs, and stack traces, in that order.
	result.WriteString("\nWarnings:\n")
	result.WriteString(toolbox.GetLatestWarnings())
	result.WriteString("\nLogs:\n")
	result.WriteString(toolbox.GetLatestLog())
	result.WriteString("\nStack traces:\n")
	result.WriteString(toolbox.GetGoroutineStacktraces())
	_, _ = w.Write(result.Bytes())
}

func (info *HandleSystemInfo) handleJson(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	NoCache(w)
	encoder := json.NewEncoder(w)
	_ = encoder.Encode(systemInfo{
		Status: platform.GetProgramStatusSummary(true),
		Stats:  misc.GetLatestDisplayValues(),
	})
}

func (info *HandleSystemInfo) Handle(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Accept")
	if strings.Contains(contentType, "application/json") {
		info.handleJson(w, r)
	} else {
		info.handlePlainText(w, r)
	}
}

func (_ *HandleSystemInfo) GetRateLimitFactor() int {
	return 2
}

func (_ *HandleSystemInfo) SelfTest() error {
	return nil
}
