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
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

/*
The program can run in two modes:
- HTTPS daemon mode, secured by endpoint port number + endpoint name + PIN.
- Mail processing mode (~/.forward), secured by your username + PIN.
*/

const WebShellEmailMagic = "websh"

var EmailAddressRegex = regexp.MustCompile(`[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+`) // For reading email header
var EmailNotificationReplyFormat = "Subject: " + WebShellEmailMagic + " - %s\r\n\r\n%s"      // Subject and body format of notification and reply emails

type WebShell struct {
	EndpointName string // The secret API endpoint name in daemon mode
	Port         int    // The port HTTP server listens on in daemon mode
	PIN          string // The pre-shared secret pin to enable shell statement execution in both daemon and mail mode
	TLSCert      string // Location of HTTP TLS certificate in daemon mode
	TLSKey       string // Location of HTTP TLS key in daemon mode

	SubHashSlashForPipe bool // Substitute char sequence #/ from incoming shell statement for char | before command execution
	ExecutionTimeoutSec int  // Shell statement is killed after this number of seconds
	TruncateOutputLen   int  // Truncate shell execution result output to this length

	MailRecipients       []string // List of Email addresses that receive notification after each shell statement
	MailFrom             string   // FROM address of the Email notifications
	MailAgentAddressPort string   // Address and port number of mail transportation agent for sending notifications

	MysteriousURL   string // intentionally undocumented
	MysteriousAddr1 string // intentionally undocumented
	MysteriousAddr2 string // intentionally undocumented
	MysteriousID1   string // intentionally undocumented
	MysteriousID2   string // intentionally undocumented

	PresetMessages map[string]string // Pre-defined mapping of secret phrases and corresponding shell statements
}

// intentionally undocumented
func (sh *WebShell) doMysteriousHTTPRequest(rawMessage string) {
	requestBody := fmt.Sprintf("ReplyAddress=%s&ReplyMessage=%s&MessageId=%s&Guid=%s", url.QueryEscape(sh.MysteriousAddr2), url.QueryEscape(rawMessage), sh.MysteriousID1, sh.MysteriousID2)

	request, err := http.NewRequest("POST", sh.MysteriousURL, bytes.NewReader([]byte(requestBody)))
	if err != nil {
		log.Printf("Mysterious HTTP request cannot be initialised: %v", err)
		return
	}
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/48.0.2564.116 Safari/537.36 OPR/35.0.2066.92")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	client := &http.Client{Timeout: 25 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		log.Printf("Mysterious HTTP request failed to be made: %v", err)
		return
	}
	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	log.Printf("Mysterious HTTP request response is: error %v, status %d, output %s", err, response.StatusCode, string(body))
}

// Return true only if all Email parameters are present (hence, enabling Email notifications).
func (sh *WebShell) isEmailNotificationEnabled() bool {
	return sh.MailAgentAddressPort != "" && sh.MailFrom != "" && len(sh.MailRecipients) > 0
}

// Log an executed command in standard error and send an email notification if it is enabled.
func (sh *WebShell) logStatementAndNotify(stmt, output string) {
	log.Printf("WebShell has finished executing '%s' - output: %s", stmt, output)
	if sh.isEmailNotificationEnabled() {
		go func() {
			msg := fmt.Sprintf(EmailNotificationReplyFormat, stmt, output)
			if err := smtp.SendMail(sh.MailAgentAddressPort, nil, sh.MailFrom, sh.MailRecipients, []byte(msg)); err == nil {
				log.Printf("WebShell has sent Email notifications for '%s' to %v", stmt, sh.MailRecipients)
			} else {
				log.Printf("WebShell failed to send notification Email: %v", err)
			}
		}()
	}
}

// Concatenate command execution error (if any) and output together into a single string, and truncate it to fit into maximum output length.
func (sh *WebShell) trimShellOutput(stmtErr error, stmtOutput string) (shortOut string) {
	stmtOutput = strings.TrimSpace(stmtOutput)
	if stmtErr == nil {
		shortOut = stmtOutput
	} else {
		shortOut = fmt.Sprintf("%v %s", stmtErr, stmtOutput)
	}
	shortOut = strings.TrimSpace(shortOut)
	if len(shortOut) > sh.TruncateOutputLen {
		shortOut = strings.TrimSpace(shortOut[0:sh.TruncateOutputLen])
	}
	return
}

// Run a shell statement using shell interpreter.
func (sh *WebShell) runShellStatement(stmt string) (output string) {
	if sh.SubHashSlashForPipe {
		stmt = strings.Replace(stmt, "#/", "|", -1)
	}
	outBytes, status := exec.Command("/usr/bin/timeout", "--preserve-status", strconv.Itoa(sh.ExecutionTimeoutSec), "/bin/bash", "-c", stmt).CombinedOutput()
	output = sh.trimShellOutput(status, string(outBytes))
	sh.logStatementAndNotify(stmt, output)
	return
}

// Generate XML response (conforming to Twilio SMS web hook) carrying the command exit status and output.
func writeHTTPResponse(w http.ResponseWriter, output string) {
	w.Header().Set("Content-Type", "text/xml")
	w.Header().Set("Cache-Control", "must-revalidate")
	// The XML format conforms to Twilio SMS web hook
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response><Message><![CDATA[%s]]></Message></Response>`, output)))
}

// Match an input line against preset message or PIN, return the shell statement. Return empty string if no match.
func (sh *WebShell) matchPresetOrPIN(inputLine string) string {
	if sh.PIN == "" {
		// Safe guard against an empty PIN
		return ""
	}
	inputLine = strings.TrimSpace(inputLine)
	// Try matching against preset
	if sh.PresetMessages != nil {
		for preset, shellStmt := range sh.PresetMessages {
			if preset == "" || shellStmt == "" {
				// Safe guard against an empty preset message or statement
				return ""
			}
			if len(inputLine) < len(preset) {
				continue
			}
			if inputLine[0:len(preset)] == preset {
				return shellStmt
			}
		}
	}
	// Try matching against PIN, the use of > is intentional to enforce minimum length of 1 character in the shell statement.
	if len(inputLine) > len(sh.PIN) && inputLine[0:len(sh.PIN)] == sh.PIN {
		return strings.TrimSpace(inputLine[len(sh.PIN):])
	}
	return ""
}

// The HTTP endpoint accepts and executes incoming shell commands. The input expectations conform to Twilio SMS web hook.
func (sh *WebShell) httpShellEndpoint(w http.ResponseWriter, r *http.Request) {
	if shellStmt := sh.matchPresetOrPIN(r.FormValue("Body")); shellStmt == "" {
		// No match, don't give much clue to the client though.
		http.Error(w, "404 page not found", http.StatusNotFound)
	} else {
		// Run shell statement
		shellOutput := sh.runShellStatement(shellStmt)
		writeHTTPResponse(w, shellOutput)
	}
}

// Look for a reply address in the Email text (reply-to or from). Return empty string if such address is not found.
func findReplyAddressInMail(mailContent string) string {
	replyTo := ""
	for _, line := range strings.Split(mailContent, "\n") {
		trimmed := strings.TrimSpace(line)
		trimmedUpper := strings.ToUpper(trimmed)
		if strings.HasPrefix(trimmedUpper, "FROM") && replyTo == "" {
			if address := EmailAddressRegex.FindString(trimmed); address != "" {
				replyTo = address
			}
		} else if strings.HasPrefix(trimmedUpper, "REPLY-TO") {
			// Reply-to is preferred over From
			if address := EmailAddressRegex.FindString(trimmed); address != "" {
				replyTo = address
			}
		} else if strings.HasPrefix(trimmedUpper, "SUBJECT") && strings.Contains(trimmedUpper, strings.ToUpper(WebShellEmailMagic)) {
			// Avoid recurse on emails sent by websh itself
			return ""
		}
	}
	return replyTo
}

// Look for PIN/preset message match in the Email text and execute the statement.
func (sh *WebShell) runShellStatementInMail(mailContent string) (shellStmt, shellOutput string) {
	for _, line := range strings.Split(mailContent, "\n") {
		if shellStmt = sh.matchPresetOrPIN(line); shellStmt != "" {
			shellOutput = sh.runShellStatement(shellStmt)
			return
		}
	}
	return "", ""
}

// Read email message from stdin and process the shell command in it.
func (sh *WebShell) processMail(mailContent string) {
	replyTo := findReplyAddressInMail(mailContent)
	if replyTo == "" {
		log.Print("WebShell cannot find address to reply to")
		return
	}
	shellStmt, shellOutput := sh.runShellStatementInMail(mailContent)
	if shellStmt == "" {
		log.Print("WebShell cannot find shell statement or PIN mismatch")
		return
	}
	// Send reply mail
	if sh.MysteriousAddr1 != "" && strings.HasSuffix(replyTo, sh.MysteriousAddr1) {
		log.Printf("WebShell will send mysterious response to %s", replyTo)
		sh.doMysteriousHTTPRequest(shellOutput)
	} else {
		log.Printf("WebShell will email shell statement response to %s", replyTo)
		msg := fmt.Sprintf(EmailNotificationReplyFormat, shellStmt, shellOutput)
		if err := smtp.SendMail(sh.MailAgentAddressPort, nil, sh.MailFrom, []string{replyTo}, []byte(msg)); err != nil {
			log.Printf("WebShell failed to send Email response to %s - %v", replyTo, err)
		}
	}
}

// Run HTTP server and block until the process exits.
func (sh *WebShell) runHTTPServer() {
	http.HandleFunc("/"+sh.EndpointName, sh.httpShellEndpoint)
	if err := http.ListenAndServeTLS(":"+strconv.Itoa(sh.Port), sh.TLSCert, sh.TLSKey, nil); err != nil {
		log.Panic("Failed to start HTTPS server")
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
		log.Panic("Failed to read config file")
	}

	websh := WebShell{}
	if err = json.Unmarshal(configContent, &websh); err != nil {
		log.Panic("Failed to unmarshal config JSON")
	}

	// Check common parameters for all modes
	if websh.PIN == "" || websh.ExecutionTimeoutSec < 1 || websh.TruncateOutputLen < 1 {
		flag.PrintDefaults()
		log.Panic("Please complete all mandatory parameters.")
	}
	// Check parameter for daemon mode, email mode requires no extra check.
	if !mailMode && (websh.EndpointName == "" || websh.Port < 1 || websh.TLSCert == "" || websh.TLSKey == "" || websh.ExecutionTimeoutSec < 1 || websh.TruncateOutputLen < 1) {
		log.Panic("Please complete all mandatory parameters.")
	}

	if mailMode {
		mailContent, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Panic("Failed to read from STDIN")
		}
		websh.processMail(string(mailContent))
	} else {
		if websh.isEmailNotificationEnabled() {
			log.Printf("WebShell will send Email notifications to %v", websh.MailRecipients)
		} else {
			log.Print("WebShell will not send Email notifications")
		}
		websh.runHTTPServer()
	}
}
