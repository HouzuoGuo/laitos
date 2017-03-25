package api

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/lalog"
	"log"
	"net/http"
)

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

// Inspect system and environment and return their information in text form. Double as a health check endpoint.
type HandleSystemInfo struct {
}

func (_ *HandleSystemInfo) MakeHandler(logger lalog.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	// Somewhat similar to healthcheck frontend
	fun := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "must-revalidate")
		// Check features
		featureErrs := cmdProc.Features.SelfTest()
		if len(featureErrs) == 0 {
			fmt.Fprint(w, "All OK.\n")
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "There are errors.\n")
		}
		// 0 - runtime info
		fmt.Fprint(w, "\nRuntime:\n")
		fmt.Fprint(w, feature.GetRuntimeInfo())
		// 1 - feature checks
		if len(featureErrs) == 0 {
			fmt.Fprint(w, "\nFeatures: OK\n")
		} else {
			for trigger, err := range featureErrs {
				fmt.Fprint(w, fmt.Sprintf("\nFeatures %s: %+v\n", trigger, err))
			}
		}
		// 2 - logs
		fmt.Fprint(w, "\nLogs:\n")
		fmt.Fprint(w, feature.GetLatestGlobalLog())
		// 3 - stack traces
		fmt.Fprint(w, "\nStack traces:\n")
		fmt.Fprint(w, feature.GetGoroutineStacktraces())
	}
	return fun, nil
}

func (_ *HandleSystemInfo) GetRateLimitFactor() int {
	return 1
}
