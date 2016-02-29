/*
A simple web server daemon enabling basic shell access via API calls.
Good for emergency system shutdown/reboot, and executing privileged/unprivileged shell code.

To call the service from command line client, run:
curl -v 'https://localhost:12321/SecretAPIEndpointName' --data-ascii 'Body=SecretPINecho hello world'

Copyright (c) 2016, Howard Guo <guohouzuo@gmail.com>
All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
- Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
- Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var NonPrintableOutput = regexp.MustCompile(`[^[:print:]]`)

var (
	endpoint string // The secret API endpoint name.
	port     int    // The port HTTP server listens on.
	pin      string // The pre-shared secret pin to enable shell command execution
	tlsCert  string // Location of HTTP TLS certificate
	tlsKey   string // Location of HTTP TLS key

	cmdTimeoutSec int // Command execution timeout
	outTruncLen   int // Truncate command output to this length

	mailRecipients []string // List of Email addresses that receive command execution notification
	mailFrom       string   // FROM address of the Email notifications
	mtaAddr        string   // Address of mail transportation agent for sending notifications
)

// Return true only if all Email parameters are present (hence, enabling Email notifications).
func isEmailNotificationEnabled() bool {
	return mtaAddr != "" && mailFrom != "" && len(mailRecipients) > 0
}

// Log an executed command in standard error and send an email notification.
func logStmt(stmt, out string, status error) {
	log.Printf("Executed '%s' - status: %v, output: %s", stmt, status, out)
	if isEmailNotificationEnabled() {
		go func() {
			msg := fmt.Sprintf("Subject: websh - %s\r\n\r\nstatus: %v output: %s", stmt, status, out)
			if err := smtp.SendMail(mtaAddr, nil, mailFrom, mailRecipients, []byte(msg)); err != nil {
				log.Printf("Failed to send notification Email: %v", err)
			}
		}()
	}
}

// Run a shell statement using shell interpreter.
func runStmt(stmt string) (out string, status error) {
	outBytes, status := exec.Command("/usr/bin/timeout", "--preserve-status", strconv.Itoa(cmdTimeoutSec), "/bin/bash", "-c", stmt).CombinedOutput()
	// Only return printable characters among the output
	out = NonPrintableOutput.ReplaceAllLiteralString(string(outBytes), "")
	return
}

// Generate XML response carrying the command exit status and output.
func writeSuccess(w http.ResponseWriter, out string, status error) {
	w.Header().Set("Content-Type", "text/xml")
	w.Header().Set("Cache-Control", "must-revalidate")
	shortOut := strings.TrimSpace(out)
	if len(shortOut) > outTruncLen {
		shortOut = out[0:outTruncLen]
	}
	// The XML format satisfies Twilio SMS API
	w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response><Message><![CDATA[%v %s]]></Message></Response>`, status, strings.TrimSpace(shortOut))))
}

// Handle Twilio API call.
func controlService(w http.ResponseWriter, r *http.Request) {
	body := strings.TrimSpace(r.FormValue("Body"))
	if len(body) < len(pin) {
		http.Error(w, "", http.StatusNotFound)
		return
	}
	cmd := strings.TrimSpace(body[len(pin):])
	if body[0:len(pin)] == pin {
		// Run arbitrary shell statement
		out, status := runStmt(cmd)
		logStmt(cmd, out, status)
		writeSuccess(w, out, status)
	} else {
		// Pin mismatch but don't give too much clue in response
		http.Error(w, "", http.StatusNotFound)
	}
}

func main() {
	// All parameters below are mandatory
	flag.StringVar(&endpoint, "endpoint", "", "The API endpoint name, keep it secret!")
	flag.IntVar(&port, "port", 0, "The port HTTPS server listens on")
	flag.StringVar(&pin, "pin", "", "The secret command prefix that authorizes command execution, keep it secret!")
	flag.StringVar(&tlsCert, "tlscert", "", "Path to HTTPS certifcate file")
	flag.StringVar(&tlsKey, "tlskey", "", "Path to HTTPS certificate key")
	flag.IntVar(&cmdTimeoutSec, "cmdtimeoutsec", 10, "Maximum time limit (in seconds) of command execution, try to keep it below 20!")
	flag.IntVar(&outTruncLen, "outtrunclen", 120, "Truncate the length of command execution output in the HTTP response, try to keep it small!")

	// Email notifications are optional
	var mailRecipientsTogether string
	flag.StringVar(&mailRecipientsTogether, "mailrecipients", "", "Comma separated list of command notification recipients, empty to disable.")
	flag.StringVar(&mailFrom, "mailfrom", "", "FROM address of command notification emails, empty to disable.")
	flag.StringVar(&mtaAddr, "mtaaddr", "", "Mail transportation agent for sending command notification emails, empty to disable.")
	flag.Parse()
	mailRecipients = strings.Split(mailRecipientsTogether, ",")

	if endpoint == "" || port < 1 || pin == "" || tlsCert == "" || tlsKey == "" || cmdTimeoutSec < 1 || outTruncLen < 1 {
		flag.PrintDefaults()
		log.Panic("Please complete all mandatory parameters.")
	}

	if isEmailNotificationEnabled() {
		log.Printf("Email notifications will be sent to %v", mailRecipients)
	} else {
		log.Print("Email notifications will not be sent")
	}
	http.HandleFunc("/"+endpoint, controlService)
	if err := http.ListenAndServeTLS(":"+strconv.Itoa(port), tlsCert, tlsKey, nil); err != nil {
		panic(err)
	}
	// ListenAndServeTLS blocks until program is interrupted or exits.
}
