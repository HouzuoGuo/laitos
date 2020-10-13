package handler

import (
	"bytes"
	"net/http"

	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/toolbox"
)

// HandleSystemInfo inspects system and application environment and returns them in text report.
type HandleSystemInfo struct {
	FeaturesToCheck    *toolbox.FeatureSet    `json:"-"` // Health check subject - features and their API keys
	CheckMailCmdRunner *mailcmd.CommandRunner `json:"-"` // Health check subject - mail processor and its mailer
	logger             lalog.Logger
}

func (info *HandleSystemInfo) Initialise(logger lalog.Logger, _ *toolbox.CommandProcessor, _ string) error {
	info.logger = logger
	return nil
}

func (info *HandleSystemInfo) Handle(w http.ResponseWriter, r *http.Request) {
	// The routine is quite similar to maintenance daemon
	w.Header().Set("Content-Type", "text/plain")
	NoCache(w)
	var result bytes.Buffer
	// Latest runtime info
	result.WriteString(toolbox.GetRuntimeInfo())
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

func (_ *HandleSystemInfo) GetRateLimitFactor() int {
	return 2
}

func (_ *HandleSystemInfo) SelfTest() error {
	return nil
}
