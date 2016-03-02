/*
A simple web server daemon enabling basic shell access via API calls.
Good for emergency system shutdown/reboot, and executing privileged/unprivileged shell code.

To call the service from command line client, run:
curl -v 'https://localhost:12321/my_secret_endpoint_name' --data-ascii 'Body=MYSECRETecho hello world'

Please note: exercise extreme caution when using this software program, inappropriate configuration will make your computer easily compromised! If you choose to use this program, I will not be responsible for any damage/potential damage caused to your computers.

Copyright (c) 2016, Howard Guo <guohouzuo@gmail.com>
All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
- Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
- Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

/*
The program can run in two modes:
- HTTPS daemon mode, secured by endpoint port number + endpoint name + PIN.
- Mail processing mode (~/.forward), secured by your username + PIN.
*/

var EmailAddressAlike = regexp.MustCompile(`[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+`) // For reading email header

var EmailNotifcationReplyFormat = "Subject: websh - %s\r\n\r\nstatus: %v output: %s" // Subject and body format of notification and reply emails, do not remove "websh" from the string.

var config struct {
	Endpoint string // The secret API endpoint name in daemon mode
	Port     int    // The port HTTP server listens on in daemon mode
	PIN      string // The pre-shared secret pin to enable shell command execution in both daemon and mail mode
	TLSCert  string // Location of HTTP TLS certificate in daemon mode
	TLSKey   string // Location of HTTP TLS key in daemon mode

	SubSectionSignForPipe bool // Substitute char § from incoming shell command for char | before command execution
	CmdTimeoutSec         int  // Command execution timeout
	OutTruncLen           int  // Truncate command output to this length

	MailRecipients []string // List of Email addresses that receive command execution notification
	MailFrom       string   // FROM address of the Email notifications
	MTAAddr        string   // Address of mail transportation agent for sending notifications
} // HTTP API daemon configuration plus the parameters also used by mail mode.

// Return true only if all Email parameters are present (hence, enabling Email notifications).
func isEmailNotificationEnabled() bool {
	return config.MTAAddr != "" && config.MailFrom != "" && len(config.MailRecipients) > 0
}

// Log an executed command in standard error and send an email notification if it is enabled.
func logStmt(stmt, out string, status error) {
	log.Printf("Executed '%s' - status: %v, output: %s", stmt, status, out)
	if isEmailNotificationEnabled() {
		go func() {
			msg := fmt.Sprintf(EmailNotifcationReplyFormat, stmt, status, out)
			if err := smtp.SendMail(config.MTAAddr, nil, config.MailFrom, config.MailRecipients, []byte(msg)); err != nil {
				log.Printf("Failed to send notification Email: %v", err)
			}
		}()
	}
}

// Run a shell statement using shell interpreter.
func runStmt(stmt string) (out string, status error) {
	if config.SubSectionSignForPipe {
		stmt = strings.Replace(stmt, "§", "|", -1)
	}
	outBytes, status := exec.Command("/usr/bin/timeout", "--preserve-status", strconv.Itoa(config.CmdTimeoutSec), "/bin/bash", "-c", stmt).CombinedOutput()
	out = string(outBytes)
	return
}

// Generate XML response (conforming to Twilio SMS web hook) carrying the command exit status and output.
func writeHTTPResponse(w http.ResponseWriter, out string, status error) {
	w.Header().Set("Content-Type", "text/xml")
	w.Header().Set("Cache-Control", "must-revalidate")
	shortOut := strings.TrimSpace(out)
	if len(shortOut) > config.OutTruncLen {
		shortOut = out[0:config.OutTruncLen]
	}
	// The XML format conforms to Twilio SMS web hook
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response><Message><![CDATA[%v %s]]></Message></Response>`, status, strings.TrimSpace(shortOut))))
}

// The HTTP endpoint accepts and executes incoming shell commands. The input expectations conform to Twilio SMS web hook.
func httpShellEndpoint(w http.ResponseWriter, r *http.Request) {
	body := strings.TrimSpace(r.FormValue("Body"))
	if len(body) < len(config.PIN) {
		// Pin mismatch but don't give too much clue in response
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}
	cmd := strings.TrimSpace(body[len(config.PIN):])
	if body[0:len(config.PIN)] == config.PIN {
		// Run arbitrary shell statement
		out, status := runStmt(cmd)
		logStmt(cmd, out, status)
		writeHTTPResponse(w, out, status)
	} else {
		// Pin mismatch but don't give too much clue in response
		http.Error(w, "404 page not found", http.StatusNotFound)
	}
}

// Read email message from stdin and process the shell command in it.
func processMail() {
	mailContent, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Panicf("Failed to read from STDIN: %v", err)
	}
	// Analyse the mail entry line by line
	lines := strings.Split(string(mailContent), "\n")
	replyTo := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Figure out reply address
		if upperLine := strings.ToUpper(trimmed); strings.HasPrefix(upperLine, "FROM") && replyTo == "" {
			if address := EmailAddressAlike.FindString(trimmed); address != "" {
				replyTo = address
			}
		} else if strings.HasPrefix(upperLine, "REPLY-TO") {
			if address := EmailAddressAlike.FindString(trimmed); address != "" {
				replyTo = address
			}
		} else if upperLine := strings.ToUpper(trimmed); strings.HasPrefix(upperLine, "SUBJECT") {
			// Avoid accidental recursive operation
			if strings.Contains(upperLine, "WEBSH") {
				return
			}
		}
		// Match PIN
		if len(trimmed) < len(config.PIN) {
			continue
		}
		if trimmed[0:len(config.PIN)] == config.PIN {
			cmd := strings.TrimSpace(trimmed[len(config.PIN):])
			out, status := runStmt(cmd)
			logStmt(cmd, out, status)
			// Send response back via Email
			msg := fmt.Sprintf(EmailNotifcationReplyFormat, cmd, status, out)
			if err := smtp.SendMail(config.MTAAddr, nil, config.MailFrom, []string{replyTo}, []byte(msg)); err != nil {
				log.Printf("Failed to send Email response back to %s - %v", replyTo, err)
			}
			return
		}
	}
}

func main() {
	var configFilePath string
	var mailMode bool
	// Read configuration file path from CLI parameter
	flag.StringVar(&configFilePath, "configfilepath", "", "Path to the configuration file")
	flag.BoolVar(&mailMode, "mailmode", false, "True if the program is processing an incoming email, false if the program is running as a daemon")
	flag.Parse()
	if configFilePath == "" {
		log.Panic("Please provide path to configuration file")
	}
	configContent, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		log.Panicf("Failed to read config file %s - %v", configFilePath, err)
	}
	if err = json.Unmarshal(configContent, &config); err != nil {
		log.Panicf("Failed to unmarshal config JSON from %s - %v", configFilePath, err)
	}

	// Check common parameters for all modes
	if config.PIN == "" || config.CmdTimeoutSec < 1 || config.OutTruncLen < 1 {
		flag.PrintDefaults()
		log.Panic("Please complete all mandatory parameters.")
	}
	// Check parameter for daemon mode, email mode requires no extra check.
	if !mailMode && (config.Endpoint == "" || config.Port < 1 || config.TLSCert == "" || config.TLSKey == "" || config.CmdTimeoutSec < 1 || config.OutTruncLen < 1) {
		log.Panic("Please complete all mandatory parameters.")
	}

	if mailMode {
		processMail()
		return
	} else {
		if isEmailNotificationEnabled() {
			log.Printf("Email notifications will be sent to %v", config.MailRecipients)
		} else {
			log.Print("Email notifications will not be sent")
		}
		http.HandleFunc("/"+config.Endpoint, httpShellEndpoint)
		if err := http.ListenAndServeTLS(":"+strconv.Itoa(config.Port), config.TLSCert, config.TLSKey, nil); err != nil {
			log.Panicf("Failed to start HTTPS service: %v", err)
		}
		return
		// ListenAndServeTLS blocks until program is interrupted or exits.
	}
}
