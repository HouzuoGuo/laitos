package api

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/HouzuoGuo/websh/frontend/common"
	"log"
	"net/http"
)

// FIXME: implement rate limit on twilio and self test handlers

// Return an http.HandlerFunc.
type HandlerFactory interface {
	MakeHandler() (http.HandlerFunc, error)
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
type FeatureSelfTest struct {
	CommandProcessor *common.CommandProcessor
}

func (hook *FeatureSelfTest) MakeHandler() (http.HandlerFunc, error) {
	if errs := hook.CommandProcessor.IsSaneForInternet(); len(errs) > 0 {
		return nil, fmt.Errorf("%+v", errs)
	}
	fun := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Cache-Control", "must-revalidate")
		errs := hook.CommandProcessor.Features.SelfTest()
		if len(errs) == 0 {
			w.Write([]byte("All OK"))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("%+v", errs)))
		}
	}
	return fun, nil
}
