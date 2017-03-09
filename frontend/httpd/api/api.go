package api

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/HouzuoGuo/websh/frontend/common"
	"log"
	"net/http"
)

const FeatureSelfTestOK = "All OK" // response body of a feature self test that all went OK

// Return an http.HandlerFunc.
type HandlerFactory interface {
	MakeHandler(*common.CommandProcessor) (http.HandlerFunc, error) // Return HTTP handler function associated with the command processor.
	GetRateLimitFactor() int                                        // Factor of how expensive the handler is to execute, 1 being most expensive.
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

func (hook *HandleFeatureSelfTest) MakeHandler(cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
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

func (hook *HandleFeatureSelfTest) GetRateLimitFactor() int {
	return 1
}
