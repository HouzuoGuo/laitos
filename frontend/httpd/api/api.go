package api

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/HouzuoGuo/laitos/env"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/frontend/dnsd"
	"github.com/HouzuoGuo/laitos/frontend/mailp"
	"github.com/HouzuoGuo/laitos/frontend/plain"
	"github.com/HouzuoGuo/laitos/frontend/smtpd"
	"github.com/HouzuoGuo/laitos/frontend/sockd"
	"github.com/HouzuoGuo/laitos/frontend/telegrambot"
	"github.com/HouzuoGuo/laitos/global"
	"log"
	"net/http"
	"strings"
)

/*
DurationStats stores statistics of duration of all HTTP requests served.
This definition should have stayed in httpd.go of httpd package, however, due to inevitable
cyclic import, the definition is made here in api package.
*/
var DurationStats = env.NewStats()

// An HTTP handler function factory.
type HandlerFactory interface {
	MakeHandler(global.Logger, *common.CommandProcessor) (http.HandlerFunc, error) // Return HTTP handler function associated with the command processor.
	GetRateLimitFactor() int                                                       // Factor of how expensive the handler is to execute, 1 being most expensive.
}

// Escape sequences in a string to make it safe for being element data.
func XMLEscape(in string) string {
	var escapeOutput bytes.Buffer
	if err := xml.EscapeText(&escapeOutput, []byte(in)); err != nil {
		log.Printf("XMLEscape: failed to escape input string - %v", err)
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
	FeaturesToCheck *feature.FeatureSet  `json:"-"` // Health check subject - features and their API keys
	MailpToCheck    *mailp.MailProcessor `json:"-"` // Health check subject - mail processor and its mailer
}

/*
GetLatestStats returns statistic information from all front-end daemons, each on their own line.
Due to inevitable cyclic import, this function is defined twice, once in api.go of api package, the other in
maintenance.go of maintenance package.
*/
func GetLatestStats() string {
	numDecimals := 2
	factor := 1000000000.0
	return fmt.Sprintf(`CmdProc: %s
DNSD TCP/UDP: %s/%s
HTTPD: %s
MAILP: %s
PLAIN TCP/UDP: %s%s
SMTPD: %s
SOCKD TCP/UDP: %s/%s
TELEGRAM BOT: %s
`,
		common.DurationStats.Format(factor, numDecimals),
		dnsd.TCPDurationStats.Format(factor, numDecimals), dnsd.UDPDurationStats.Format(factor, numDecimals),
		DurationStats.Format(factor, numDecimals),
		mailp.DurationStats.Format(factor, numDecimals),
		plain.TCPDurationStats.Format(factor, numDecimals), plain.UDPDurationStats.Format(factor, numDecimals),
		smtpd.DurationStats.Format(factor, numDecimals),
		sockd.TCPDurationStats.Format(factor, numDecimals), sockd.UDPDurationStats.Format(factor, numDecimals),
		telegrambot.DurationStats.Format(factor, numDecimals))
}

func (info *HandleSystemInfo) MakeHandler(logger global.Logger, _ *common.CommandProcessor) (http.HandlerFunc, error) {
	// Somewhat similar to health-check frontend
	fun := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		NoCache(w)
		if !WarnIfNoHTTPS(r, w) {
			return
		}
		// Check features and mail processor
		featureErrs := make(map[feature.Trigger]error)
		if info.FeaturesToCheck != nil {
			featureErrs = info.FeaturesToCheck.SelfTest()
		}
		var mailpErr error
		if info.MailpToCheck != nil {
			mailpErr = info.MailpToCheck.SelfTest()
		}
		allOK := len(featureErrs) == 0 && mailpErr == nil
		// Compose mail body
		if allOK {
			fmt.Fprint(w, "All OK\n")
		} else {
			fmt.Fprint(w, "There are errors!!!\n")
		}
		// Runtime info
		fmt.Fprint(w, feature.GetRuntimeInfo())
		// Statistics
		fmt.Fprint(w, "\nStatistics low/avg/high/total(count) seconds:\n")
		fmt.Fprint(w, GetLatestStats())
		// Feature checks
		if len(featureErrs) == 0 {
			fmt.Fprint(w, "\nFeatures: OK\n")
		} else {
			for trigger, err := range featureErrs {
				fmt.Fprintf(w, "\nFeatures %s: %+v\n", trigger, err)
			}
		}
		// Mail processor checks
		if mailpErr == nil {
			fmt.Fprint(w, "\nMail processor: OK\n")
		} else {
			fmt.Fprintf(w, "\nMail processor: %v\n", mailpErr)
		}
		// Warnings, logs, and stack traces
		fmt.Fprint(w, "\nWarnings:\n")
		fmt.Fprint(w, feature.GetLatestWarnings())
		fmt.Fprint(w, "\nLogs:\n")
		fmt.Fprint(w, feature.GetLatestLog())
		fmt.Fprint(w, "\nStack traces:\n")
		fmt.Fprint(w, feature.GetGoroutineStacktraces())
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
