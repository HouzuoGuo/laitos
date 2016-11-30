package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

type APIServer struct {
	MessageEndpoint     string // The secret API endpoint name for messaging in daemon mode
	VoiceMLEndpoint     string // The secret API endpoint name that serves TwiML voice script in daemon mode
	VoiceProcEndpoint   string // The secret API endpoint name that responds to TwiML voice script
	VoiceEndpointPrefix string // The HTTP scheme and/or host name and/or URL prefix that will correctly construct URLs leading to ML and Proc endpoints
	ServerPort          int    // The port HTTP server listens on in daemon mode
	TLSCert             string // Location of HTTP TLS certificate in daemon mode
	TLSKey              string // Location of HTTP TLS key in daemon mode

	Command CommandRunner
}

// Make a short pause of random duration between 2 and 4 seconds.
func PauseFewSecs() {
	time.Sleep(time.Millisecond * time.Duration(2000+rand.Intn(2000)))
}

// Validate configuration, make sure they look good.
func (api *APIServer) CheckConfig() error {
	if err := api.Command.CheckConfig(); err != nil {
		return err
	} else if api.ServerPort < 1 || api.TLSCert == "" || api.TLSKey == "" {
		return errors.New("Please complete server port and TLS configuration")
	}
	return nil
}

// Run HTTP server and block until the process exits.
func (web *APIServer) Run() {
	http.HandleFunc("/"+web.MessageEndpoint, web.httpAPIMessage)
	http.HandleFunc("/"+web.VoiceMLEndpoint, web.httpAPIVoiceGreeting)
	http.HandleFunc("/"+web.VoiceProcEndpoint, web.httpAPIVoiceMessage)
	if err := http.ListenAndServeTLS(":"+strconv.Itoa(web.ServerPort), web.TLSCert, web.TLSKey, nil); err != nil {
		log.Panicf("Failed to start HTTPS server - %v", err)
	}
}

// Look for command to execute from request body. The request/response content conform to Twilio SMS hook requirements.
func (web *APIServer) httpAPIMessage(w http.ResponseWriter, r *http.Request) {
	if cmd := web.Command.FindCommand(r.FormValue("Body")); cmd == "" {
		// No match, don't give much clue to the client though.
		PauseFewSecs()
		http.Error(w, "404 page not found", http.StatusNotFound)
	} else {
		output := web.Command.RunCommand(cmd, true)
		var escapeOutput bytes.Buffer
		if err := xml.EscapeText(&escapeOutput, []byte(output)); err != nil {
			log.Printf("XML escape failed - %v", err)
		}
		w.Header().Set("Content-Type", "text/xml")
		w.Header().Set("Cache-Control", "must-revalidate")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response><Message>%s</Message></Response>
`, escapeOutput.String())))
	}
}

// The HTTP Voice mark-up endpoint returns TwiML voice script.
func (web *APIServer) httpAPIVoiceGreeting(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml")
	w.Header().Set("Cache-Control", "must-revalidate")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s%s" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say>Hello</Say>
    </Gather>
</Response>
`, web.VoiceEndpointPrefix, web.VoiceProcEndpoint)))
}

// The HTTP voice processing endpoint reads DTMF (input from Twilio request) and translates it into a statement and execute.
func (web *APIServer) httpAPIVoiceMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml")
	w.Header().Set("Cache-Control", "must-revalidate")
	w.WriteHeader(http.StatusOK)
	digits := r.FormValue("Digits")
	decodedLetters := DTMFDecode(digits)
	log.Printf("Caller typed %d digits that decode into %d letters", len(digits), len(decodedLetters))
	if cmd := web.Command.FindCommand(decodedLetters); cmd == "" {
		// PIN mismatch
		PauseFewSecs()
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
	<Say>Sorry</Say>
	<Hangup/>
</Response>
`))
	} else {
		// Speak the command result and repeat once
		output := web.Command.RunCommand(cmd, true)
		var escapeOutput bytes.Buffer
		if err := xml.EscapeText(&escapeOutput, []byte(output)); err != nil {
			log.Printf("XML escape failed - %v", err)
		}
		w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s%s" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say>%s over</Say>
    </Gather>
</Response>
`, web.VoiceEndpointPrefix, web.VoiceProcEndpoint, escapeOutput.String())))
	}
}
