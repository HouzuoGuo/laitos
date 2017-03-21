package api

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/HouzuoGuo/laitos/env"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/lalog"
	"log"
	"net/http"
	"runtime"
	"runtime/pprof"
)

const FeatureSelfTestOK = "All OK" // response body of a feature self test that all went OK

// An HTTP handler function factory.
type HandlerFactory interface {
	MakeHandler(lalog.Logger, *common.CommandProcessor) (http.HandlerFunc, error) // Return HTTP handler function associated with the command processor.
	GetRateLimitFactor() int                                                      // Factor of how expensive the handler is to execute, 1 being most expensive.
}

// Escape sequences in a string to make it safe for being element data.
func XMLEscape(in string) string {
	var escapeOutput bytes.Buffer
	if err := xml.EscapeText(&escapeOutput, []byte(in)); err != nil {
		log.Printf("XMLEscape: failed - %v", err)
	}
	return escapeOutput.String()
}

// Implement health check end-point for all features configured in the command processor.
type HandleFeatureSelfTest struct {
}

func (_ *HandleFeatureSelfTest) MakeHandler(logger lalog.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	fun := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "must-revalidate")
		errs := cmdProc.Features.SelfTest()
		if len(errs) == 0 {
			w.Write([]byte(FeatureSelfTestOK))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			var lines bytes.Buffer
			for triggerPrefix, err := range errs {
				lines.WriteString(fmt.Sprintf("%s: %v<br/>\n", triggerPrefix, err))
			}
			w.Write([]byte(lines.String()))
		}
	}
	return fun, nil
}

func (_ *HandleFeatureSelfTest) GetRateLimitFactor() int {
	return 1
}

// Inspect system and environment and return their information in text form.
type HandleSystemInfo struct {
}

func (_ *HandleSystemInfo) MakeHandler(logger lalog.Logger, _ *common.CommandProcessor) (http.HandlerFunc, error) {
	fun := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "must-revalidate")
		w.Write([]byte(fmt.Sprintf("Public IP: %s\n", env.GetPublicIP())))
		w.Write([]byte(fmt.Sprintf("GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))))
		w.Write([]byte("\nLog: \n"))
		lalog.GlobalRingBuffer.Iterate(func(_ uint64, msg string) bool {
			w.Write([]byte(fmt.Sprintf("%s\n", msg)))
			return true
		})
		w.Write([]byte("\nGoroutines: \n"))
		pprof.Lookup("goroutine").WriteTo(w, 1)
	}
	return fun, nil
}

func (_ *HandleSystemInfo) GetRateLimitFactor() int {
	return 1
}
