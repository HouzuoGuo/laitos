package handler

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"github.com/HouzuoGuo/laitos/daemon/autounlock"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/daemon/plainsocket"
	"github.com/HouzuoGuo/laitos/daemon/smtpd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/daemon/sockd"
	"github.com/HouzuoGuo/laitos/daemon/telegrambot"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
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
Auto-unlock events:   %s
Outstanding mails:    %d KB
`,
		common.DurationStats.Format(factor, numDecimals),
		dnsd.TCPDurationStats.Format(factor, numDecimals), dnsd.UDPDurationStats.Format(factor, numDecimals),
		DurationStats.Format(factor, numDecimals),
		mailcmd.DurationStats.Format(factor, numDecimals),
		plainsocket.TCPDurationStats.Format(factor, numDecimals), plainsocket.UDPDurationStats.Format(factor, numDecimals),
		smtpd.DurationStats.Format(factor, numDecimals),
		sockd.TCPDurationStats.Format(factor, numDecimals), sockd.UDPDurationStats.Format(factor, numDecimals),
		telegrambot.DurationStats.Format(factor, numDecimals),
		autounlock.UnlockStats.Format(factor, numDecimals),
		inet.OutstandingMailBytes/1024)
}
