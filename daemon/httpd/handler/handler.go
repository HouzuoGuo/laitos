package handler

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"strings"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/lalog"
)

// An HTTP handler function factory.
type Handler interface {
	// Initialise prepares internal handler states and optionally memorises the logger and command processor instance.
	Initialise(lalog.Logger, *common.CommandProcessor) error

	// GetHandler is the HTTP handler implementation that uses handler internal states to serve API requests.
	Handle(http.ResponseWriter, *http.Request)

	// GetRateLimitFactor returns how expensive the handler is to execute on a scale from 1 (most expensive) to infinity (least expensive).
	GetRateLimitFactor() int

	// SelfTest validates configuration such as connectivity to external service. It may work only after Initialise() succeeds.
	SelfTest() error
}

// Escape sequences in a string to make it safe for being element data.
func XMLEscape(in string) string {
	var escapeOutput bytes.Buffer
	if err := xml.EscapeText(&escapeOutput, []byte(in)); err != nil {
		lalog.DefaultLogger.Warning("XMLEscape", "", err, "failed to escape input string")
	}
	return escapeOutput.String()
}

// Set response headers to prevent client from caching HTTP request or response.
func NoCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
}

/*
GetRealClientIP returns the IP of HTTP client who made the request.
Usually, the return value is identical to IP portion of RemoteAddr, but if there is an nginx
proxy in front of web server (typical for Elastic Beanstalk), the return value will be client IP
address read from header "X-Real-Ip".
*/
func GetRealClientIP(r *http.Request) string {
	ip := r.RemoteAddr[:strings.LastIndexByte(r.RemoteAddr, ':')]
	if strings.HasPrefix(ip, "127.") {
		if realIP := r.Header["X-Real-Ip"]; realIP != nil && len(realIP) > 0 {
			ip = realIP[0]
		}
	}
	return ip
}
