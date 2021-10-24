package handler

import (
	"net/http"
	"strconv"

	"github.com/HouzuoGuo/laitos/daemon/httpd/middleware"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

// HandleLatestRequestsInspector turns on/off the recording of the latest HTTP
// requests processed by HTTP daemons, and displays them for inspection.
type HandleLatestRequestsInspector struct {
}

// Initialise the handler instance. This function always returns nil.
func (_ *HandleLatestRequestsInspector) Initialise(_ lalog.Logger, _ *toolbox.CommandProcessor, _ string) error {
	return nil
}

// GetRateLimitFactor returns the rate limit multiplication factor of this
// handler, which is integer 1.
func (_ *HandleLatestRequestsInspector) GetRateLimitFactor() int {
	return 1
}

// SelfTest always returns nil.
func (_ *HandleLatestRequestsInspector) SelfTest() error {
	return nil
}

// Handle shows various parameters about the request (e.g. headers, body, etc) in a plain text response.
func (_ *HandleLatestRequestsInspector) Handle(w http.ResponseWriter, r *http.Request) {
	NoCache(w)
	w.Header().Set("Content-Type", "text/html")
	enable, err := strconv.ParseBool(r.FormValue("e"))
	if err != nil {
		// Just browsing - print out the collected requests.
		for _, req := range middleware.LatestRequests.GetAll() {
			_, _ = w.Write([]byte("<pre>"))
			_, _ = w.Write([]byte(req))
			_, _ = w.Write([]byte("</pre><hr>\n"))
		}
	} else if enable {
		middleware.EnableLatestRequestsRecording = true
		_, _ = w.Write([]byte("Start memorising latest requests."))
	} else {
		// Stop memorising latest requests.
		middleware.EnableLatestRequestsRecording = false
		middleware.LatestRequests.Clear()
		_, _ = w.Write([]byte("Stop memorising latest requests."))
	}
}
