package httpd

import (
	"github.com/HouzuoGuo/websh/frontend/common"
	"net/http"
)

// Create common HTTP handler functions.
type APIFactory struct {
	CommandProcessor common.CommandProcessor
}

func (fact *APIFactory) TwilioSMSHandler(w http.ResponseWriter, r *http.Request) {

}

func (fact *APIFactory) TwilioCallGreetingHandler(w http.ResponseWriter, r *http.Request) {

}

func (fact *APIFactory) TwilioCallCommandHandler(w http.ResponseWriter, r *http.Request) {

}
