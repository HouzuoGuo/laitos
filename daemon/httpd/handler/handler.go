package handler

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"strings"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

// An HTTP handler function factory.
type Handler interface {
	// Initialise prepares internal handler states and optionally memorises the logger and command processor instance.
	Initialise(lalog.Logger, *toolbox.CommandProcessor) error

	// GetHandler is the HTTP handler implementation that uses handler internal states to serve API requests.
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
		lalog.DefaultLogger.Warning("XMLEscape", "", err, "failed to escape XML")
	}
	return out.String()
}

// Set response headers to prevent client from caching HTTP request or response.
func NoCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
}

/*
GetRealClientIP returns the IP of HTTP client that initiated the HTTP request.
Usually, the return value is identical to IP portion of RemoteAddr, but if there is a proxy server in between,
such as a load balancer or LAN proxy, the return value will be the client IP address read from header
"X-Real-Ip" (preferred) or "X-Forwarded-For".
*/
func GetRealClientIP(r *http.Request) string {
	ip := r.RemoteAddr[:strings.LastIndexByte(r.RemoteAddr, ':')]
	if strings.HasPrefix(ip, "127.") {
		if realIP := r.Header["X-Real-Ip"]; len(realIP) > 0 {
			ip = realIP[0]
		} else if forwardedFor := r.Header["X-Forwarded-For"]; len(forwardedFor) > 0 {
			// X-Forwarded-For value looks like "1.1.1.1[, 2.2.2.2, 3.3.3.3 ...]" where the first IP is the client IP
			split := strings.Split(forwardedFor[0], ",")
			if len(split) > 0 {
				ip = split[0]
			}
		}
	}
	return ip
}
