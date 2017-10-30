package api

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
	"sync"
)

/*
DurationStats stores statistics of duration of all HTTP requests served.
This definition should have stayed in httpd.go of httpd package, however, due to inevitable
cyclic import, the definition is made here in api package.
*/
var DurationStats = misc.NewStats()

// An HTTP handler function factory.
type HandlerFactory interface {
	MakeHandler(misc.Logger, *common.CommandProcessor) (http.HandlerFunc, error) // Return HTTP handler function associated with the command processor.
	GetRateLimitFactor() int                                                     // Factor of how expensive the handler is to execute, 1 being most expensive.
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

// Inspect system and environment and return their information in text form. Double as a health check endpoint.
type HandleSystemInfo struct {
	FeaturesToCheck    *toolbox.FeatureSet    `json:"-"` // Health check subject - features and their API keys
	CheckMailCmdRunner *mailcmd.CommandRunner `json:"-"` // Health check subject - mail processor and its mailer
}

/*
GetLatestStats returns statistic information from all front-end daemons, each on their own line.
Due to inevitable cyclic import, this function is defined twice, once in api.go of api package, the other in
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

func (info *HandleSystemInfo) MakeHandler(logger misc.Logger, _ *common.CommandProcessor) (http.HandlerFunc, error) {
	// The routine is quite similar to maintenance daemon
	fun := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		NoCache(w)
		if !WarnIfNoHTTPS(r, w) {
			return
		}
		// Do two checks in parallel - features and mail command runner
		var featureErrs map[toolbox.Trigger]error
		var mailCmdRunnerErr error
		waitAllChecks := new(sync.WaitGroup)
		waitAllChecks.Add(2) // will wait for features and mail command runner
		go func() {
			// Feature self test - the routine itself also uses concurrency internally
			featureErrs = info.FeaturesToCheck.SelfTest()
			waitAllChecks.Done()
		}()
		go func() {
			// Mail command runner test - the routine itself also uses concurrency internally
			if info.CheckMailCmdRunner != nil {
				mailCmdRunnerErr = info.CheckMailCmdRunner.SelfTest()
			}
			waitAllChecks.Done()
		}()
		waitAllChecks.Wait()

		// Results are ready, time to compose mail body.
		allOK := len(featureErrs) == 0 && mailCmdRunnerErr == nil
		var result bytes.Buffer
		if allOK {
			result.WriteString("All OK\n")
		} else {
			result.WriteString("There are errors!!!\n")
		}
		// Latest runtime info
		result.WriteString(toolbox.GetRuntimeInfo())
		// Latest stats
		result.WriteString("\nDaemon stats - low/avg/high/total seconds and (count):\n")
		result.WriteString(GetLatestStats())
		// Feature check results
		if len(featureErrs) == 0 {
			result.WriteString("\nFeatures: OK\n")
		} else {
			for trigger, err := range featureErrs {
				result.WriteString(fmt.Sprintf("\nFeatures %s: %+v\n", trigger, err))
			}
		}
		// Mail command runner check results
		if mailCmdRunnerErr == nil {
			result.WriteString("\nMail processor: OK\n")
		} else {
			result.WriteString(fmt.Sprintf("\nMail processor: %v\n", mailCmdRunnerErr))
		}
		// Warnings, logs, and stack traces, in that order.
		result.WriteString("\nWarnings:\n")
		result.WriteString(toolbox.GetLatestWarnings())
		result.WriteString("\nLogs:\n")
		result.WriteString(toolbox.GetLatestLog())
		result.WriteString("\nStack traces:\n")
		result.WriteString(toolbox.GetGoroutineStacktraces())
		w.Write(result.Bytes())
	}
	return fun, nil
}

func (_ *HandleSystemInfo) GetRateLimitFactor() int {
	return 2
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
