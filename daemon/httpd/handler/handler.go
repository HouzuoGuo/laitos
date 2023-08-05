package handler

import (
	"bytes"
	"encoding/xml"
	"net/http"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

// An HTTP handler function factory.
type Handler interface {
	// Initialise prepares internal handler states and optionally memorises the logger and command processor instance.
	Initialise(logger *lalog.Logger, cmdProc *toolbox.CommandProcessor, stripURLPrefixFromResponse string) error

	// Handle is the HTTP handler implementation that uses handler internal states to serve API requests.
	Handle(http.ResponseWriter, *http.Request)

	// GetRateLimitFactor returns how expensive the handler is to execute on a scale from 1 (most expensive) to infinity (least expensive).
	GetRateLimitFactor() int

	// SelfTest validates configuration such as connectivity to external service. It may work only after Initialise() succeeds.
	SelfTest() error
}

// XMLEscape returns properly escaped XML equivalent of the plain text input.
func XMLEscape(in string) string {
	out := new(bytes.Buffer)
	err := xml.EscapeText(out, []byte(in))
	if err != nil {
		lalog.DefaultLogger.Warning("", err, "failed to escape XML")
	}
	return out.String()
}

// Set response headers to prevent client from caching HTTP request or response.
func NoCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
}
