package handler

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/daemon/plainsocket"
	"github.com/HouzuoGuo/laitos/daemon/smtpd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/daemon/sockd"
	"github.com/HouzuoGuo/laitos/daemon/telegrambot"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/toolbox"
	"net/http"
	"strings"
)

/*
DurationStats stores statistics of duration of all HTTP requests served.
This definition should have stayed in httpd.go of httpd package, however, due to inevitable
cyclic import, the definition is made here in handler package.
*/
var DurationStats = misc.NewStats()

// An HTTP handler function factory.
type Handler interface {
	// Initialise prepares internal handler states and optionally memorises the logger and command processor instance.
	Initialise(misc.Logger, *common.CommandProcessor) error

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
		misc.DefaultLogger.Warningf("XMLEscape", "", err, "failed to escape input string")
	}
	return escapeOutput.String()
}

// Set response headers to prevent client from caching HTTP request or response.
func NoCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
}

/*
If request came in HTTP instead of HTTPS, asks client to confirm the request via a dummy basic authentication request.
Return true only if caller should continue processing the request.
*/
func WarnIfNoHTTPS(r *http.Request, w http.ResponseWriter) bool {
	if r.TLS == nil {
		if _, _, ok := r.BasicAuth(); !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="You are not using HTTPS. Enter any user/password to continue."`)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte{})
			return false
		}
	}
	return true
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

/*
GetLatestStats returns statistic information from all front-end daemons, each on their own line.
Due to inevitable cyclic import, this function is defined twice, once in handler.go of handler package, the other in
maintenance.go of maintenance package.
*/
func GetLatestStats() string {
	numDecimals := 2
	factor := 1000000000.0
	return fmt.Sprintf(`Web and bot commands: %s
DNS server  TCP|UDP:  %s | %s
Web servers:          %s
Mail commands:        %s
Text server TCP|UDP:  %s | %s
Mail server:          %s
Sock server TCP|UDP:  %s | %s
Telegram commands:    %s
`,
		common.DurationStats.Format(factor, numDecimals),
		dnsd.TCPDurationStats.Format(factor, numDecimals), dnsd.UDPDurationStats.Format(factor, numDecimals),
		DurationStats.Format(factor, numDecimals),
		mailcmd.DurationStats.Format(factor, numDecimals),
		plainsocket.TCPDurationStats.Format(factor, numDecimals), plainsocket.UDPDurationStats.Format(factor, numDecimals),
		smtpd.DurationStats.Format(factor, numDecimals),
		sockd.TCPDurationStats.Format(factor, numDecimals), sockd.UDPDurationStats.Format(factor, numDecimals),
		telegrambot.DurationStats.Format(factor, numDecimals))
}

// Inspect system and environment and return their information in text form. Double as a health check endpoint.
type HandleSystemInfo struct {
	FeaturesToCheck    *toolbox.FeatureSet    `json:"-"` // Health check subject - features and their API keys
	CheckMailCmdRunner *mailcmd.CommandRunner `json:"-"` // Health check subject - mail processor and its mailer
	logger             misc.Logger
}

func (info *HandleSystemInfo) Initialise(logger misc.Logger, _ *common.CommandProcessor) error {
	info.logger = logger
	return nil
}

func (info *HandleSystemInfo) Handle(w http.ResponseWriter, r *http.Request) {
	// The routine is quite similar to maintenance daemon
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	NoCache(w)
	if !WarnIfNoHTTPS(r, w) {
		return
	}
	var result bytes.Buffer
	// Latest runtime info
	result.WriteString(toolbox.GetRuntimeInfo())
	// Latest stats
	result.WriteString("\nDaemon stats - low/avg/high/total seconds and (count):\n")
	result.WriteString(GetLatestStats())
	// Warnings, logs, and stack traces, in that order.
	result.WriteString("\nWarnings:\n")
	result.WriteString(toolbox.GetLatestWarnings())
	result.WriteString("\nLogs:\n")
	result.WriteString(toolbox.GetLatestLog())
	result.WriteString("\nStack traces:\n")
	result.WriteString(toolbox.GetGoroutineStacktraces())
	w.Write(result.Bytes())
}

func (_ *HandleSystemInfo) GetRateLimitFactor() int {
	return 2
}

func (_ *HandleSystemInfo) SelfTest() error {
	return nil
}
