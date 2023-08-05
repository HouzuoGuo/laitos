package handler

import (
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

// HandleRequestInspector is an HTTP handler that displays various parameters about the request (e.g. headers, body, etc) in
// a plain text response.
type HandleRequestInspector struct {
}

// Initialise the handler instance. This function always returns nil.
func (_ *HandleRequestInspector) Initialise(_ *lalog.Logger, _ *toolbox.CommandProcessor, _ string) error {
	return nil
}

// GetRateLimitFactor returns the rate limit multiplication factor of this handler, which is integer 1.
func (_ *HandleRequestInspector) GetRateLimitFactor() int {
	return 1
}

// SelfTest always returns nil.
func (_ *HandleRequestInspector) SelfTest() error {
	return nil
}

// Handle shows various parameters about the request (e.g. headers, body, etc) in a plain text response.
func (_ *HandleRequestInspector) Handle(w http.ResponseWriter, r *http.Request) {
	NoCache(w)
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to dump request - %v", err), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(dump)
}
