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
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
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
const WolframAlphaTrigger = "#w"                                                             // Message prefix that triggers WolframAlpha query

type WebShell struct {
	MessageEndpoint   string // The secret API endpoint name for messaging in daemon mode
	VoiceMLEndpoint   string // The secret API endpoint name that serves TwiML voice script in daemon mode
	VoiceProcEndpoint string // The secret API endpoint name that responds to TwiML voice script
	ServerPort        int    // The port HTTP server listens on in daemon mode
	PIN               string // The pre-shared secret pin to enable shell statement execution in both daemon and mail mode
	TLSCert           string // Location of HTTP TLS certificate in daemon mode
	TLSKey            string // Location of HTTP TLS key in daemon mode

	SubHashSlashForPipe bool // Substitute char sequence #/ from incoming shell statement for char | before command execution
	WebTimeoutSec       int  // When reached from web API, WolframAlpha query/shell statement is killed after this number of seconds.
	WebTruncateLen      int  // When reached from web API, truncate statement execution result to this length.

	MailTimeoutSec       int      // When reached from mail API, WolframAlpha query/shell statement is killed after this number of seconds.
	MailTruncateLen      int      // When reached from mail API, truncate statement execution result to this length.
	MailRecipients       []string // List of Email addresses that receive notification after each shell statement
	MailFrom             string   // FROM address of the Email notifications
	MailAgentAddressPort string   // Address and port number of mail transportation agent for sending notifications

	MysteriousURL   string // intentionally undocumented
	MysteriousAddr1 string // intentionally undocumented
	MysteriousAddr2 string // intentionally undocumented
	MysteriousID1   string // intentionally undocumented
	MysteriousID2   string // intentionally undocumented

	WolframAlphaAppID string // WolframAlpha application ID for consuming its APIs

	PresetMessages map[string]string // Pre-defined mapping of secret phrases and corresponding shell statements
}

// intentionally undocumented
func (sh *WebShell) doMysteriousHTTPRequest(rawMessage string) {
	requestBody := fmt.Sprintf("ReplyAddress=%s&ReplyMessage=%s&MessageId=%s&Guid=%s", url.QueryEscape(sh.MysteriousAddr2), url.QueryEscape(rawMessage), sh.MysteriousID1, sh.MysteriousID2)

	request, err := http.NewRequest("POST", sh.MysteriousURL, bytes.NewReader([]byte(requestBody)))
	if err != nil {
		log.Printf("MysteriousShell cannot initialise HTTP request for '%s': %v", rawMessage, err)
		return
	}
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/48.0.2564.116 Safari/537.36 OPR/35.0.2066.92")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	client := &http.Client{Timeout: 25 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		log.Printf("MysteriousShell failed to make HTTP request for '%s': %v", rawMessage, err)
		return
	}
	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	log.Printf("MysteriousShell got response for '%s': error %v, status %d, output %s", rawMessage, err, response.StatusCode, string(body))
}

// Extract "pods" from WolframAlpha API response in XML.
func (sh *WebShell) extractWolframAlphaResponseText(xmlBody []byte) string {
	type SubPod struct {
		TextInfo string `xml:"plaintext"`
		Title    string `xml:"title,attr"`
	}
	type Pod struct {
		SubPods []SubPod `xml:"subpod"`
		Title   string   `xml:"title,attr"`
	}
	type QueryResult struct {
		Pods []Pod `xml:"pod"`
	}
	var result QueryResult
	if err := xml.Unmarshal(xmlBody, &result); err != nil {
		return err.Error()
	}
	var outBuf bytes.Buffer
	for _, pod := range result.Pods {
		for _, subPod := range pod.SubPods {
			outBuf.WriteString(strings.TrimSpace(subPod.TextInfo))
			outBuf.WriteRune('#')
		}
	}
	return outBuf.String()
}

// Call WolframAlpha API.
func (sh *WebShell) doWolframAlphaRequest(timeoutSec int, query string) string {
	request, err := http.NewRequest(
		"GET",
		fmt.Sprintf("https://api.wolframalpha.com/v2/query?appid=%s&input=%s&format=plaintext", sh.WolframAlphaAppID, url.QueryEscape(query)),
		bytes.NewReader([]byte{}))
	if err != nil {
		log.Printf("Failed to initialise WolframAlpha HTTP request for '%s': %v", query, err)
		return ""
	}

	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	response, err := client.Do(request)
	if err != nil {
		log.Printf("Failed to make WolframAlpha request for '%s': %v", query, err)
		return ""
	}
	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	textResponse := sh.extractWolframAlphaResponseText(body)
	log.Printf("Got response from WolframAlpha for '%s': error %v, status %d, output %s", query, err, response.StatusCode, textResponse)
	return textResponse
}

// Return true only if all Email parameters are present (hence, enabling Email notifications).
func (sh *WebShell) isEmailNotificationEnabled() bool {
	return sh.MailAgentAddressPort != "" && sh.MailFrom != "" && len(sh.MailRecipients) > 0
}

// Log an executed command in standard error and send an email notification if it is enabled.
func (sh *WebShell) logStatementAndNotify(stmt, output string) {
	log.Printf("Websh has finished executing '%s' - output: %s", stmt, output)
	if sh.isEmailNotificationEnabled() {
		go func() {
			msg := fmt.Sprintf(EmailNotificationReplyFormat, stmt, output)
			if err := smtp.SendMail(sh.MailAgentAddressPort, nil, sh.MailFrom, sh.MailRecipients, []byte(msg)); err == nil {
				log.Printf("Websh has sent Email notifications for '%s' to %v", stmt, sh.MailRecipients)
			} else {
				log.Printf("Websh failed to send notification Email: %v", err)
			}
		}()
	}
}

// Concatenate command execution error (if any) and output together into a single string, and truncate it to fit into maximum output length.
func (sh *WebShell) lintOutput(outErr error, outText string, maxOutLen int, squeezeIntoOneLine, truncateToLen bool) (out string) {
	outLines := make([]string, 0, 8)
	if outErr != nil {
		for _, line := range strings.Split(fmt.Sprint(outErr), "\n") {
			outLines = append(outLines, strings.TrimSpace(line))
		}
	}
	for _, line := range strings.Split(outText, "\n") {
		outLines = append(outLines, strings.TrimSpace(line))
	}
	if squeezeIntoOneLine {
		out = strings.Join(outLines, ";")
	} else {
		out = strings.Join(outLines, "\n")
	}
	if truncateToLen && len(out) > maxOutLen {
		out = out[0:maxOutLen]
	}
	if out == "" {
		return "EMPTY OUTPUT"
	}
	return strings.TrimSpace(out)
}

// Run a WolframAlpha query or shell statement using shell interpreter.
func (sh *WebShell) runStatement(stmt string, timeoutSec, maxOutLen int, squeezeIntoOneLine, truncateToLen bool) (output string) {
	log.Printf("Websh will run statement '%s'", stmt)
	if strings.HasPrefix(stmt, WolframAlphaTrigger) {
		output = sh.lintOutput(nil, sh.doWolframAlphaRequest(timeoutSec, stmt[len(WolframAlphaTrigger):]), maxOutLen, squeezeIntoOneLine, truncateToLen)
	} else {
		if sh.SubHashSlashForPipe {
			stmt = strings.Replace(stmt, "#/", "|", -1)
		}
		outBytes, status := exec.Command("/usr/bin/timeout", "--preserve-status", strconv.Itoa(timeoutSec), "/bin/bash", "-c", stmt).CombinedOutput()
		output = sh.lintOutput(status, string(outBytes), maxOutLen, squeezeIntoOneLine, truncateToLen)
	}
	sh.logStatementAndNotify(stmt, output)
	return
}

// Generate XML response (conforming to Twilio SMS web hook) carrying the command exit status and output.
func writeHTTPResponse(w http.ResponseWriter, output string) {
	w.Header().Set("Content-Type", "text/xml")
	w.Header().Set("Cache-Control", "must-revalidate")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response><Message><![CDATA[%s]]></Message></Response>`, output)))
}

// Match an input line against preset message or PIN, return the statement. Return empty string if no match.
func (sh *WebShell) matchPresetOrPIN(inputLine string) string {
	if sh.PIN == "" {
		// Safe guard against an empty PIN
		return ""
	}
	inputLine = strings.TrimSpace(inputLine)
	// Try matching against preset
	if sh.PresetMessages != nil {
		for preset, stmt := range sh.PresetMessages {
			if preset == "" || stmt == "" {
				// Safe guard against an empty preset message or statement
				return ""
			}
			if len(inputLine) < len(preset) {
				continue
			}
			if inputLine[0:len(preset)] == preset {
				return stmt
			}
		}
	}
	// Try matching against PIN, the use of > is intentional to enforce minimum length of 1 character in the shell statement.
	if len(inputLine) > len(sh.PIN) && inputLine[0:len(sh.PIN)] == sh.PIN {
		return strings.TrimSpace(inputLine[len(sh.PIN):])
	}
	return ""
}

// Look for a reply address in the Email text (reply-to or from). Return empty string if such address is not found.
func findSubjectAndReplyAddressInMail(mailContent string) (subject string, contentType string, replyTo string) {
	for _, line := range strings.Split(mailContent, "\n") {
		trimmed := strings.TrimSpace(line)
		trimmedUpper := strings.ToUpper(trimmed)
		if strings.HasPrefix(trimmedUpper, "FROM:") && replyTo == "" {
			if address := EmailAddressRegex.FindString(trimmed); address != "" {
				replyTo = address
			}
		} else if strings.HasPrefix(trimmedUpper, "REPLY-TO:") {
			// Reply-to is preferred over From
			if address := EmailAddressRegex.FindString(trimmed); address != "" {
				replyTo = address
			}
		} else if strings.HasPrefix(trimmedUpper, "SUBJECT:") {
			if strings.Contains(trimmedUpper, strings.ToUpper(WebShellEmailMagic)) {
				// Avoid recurse on emails sent by websh itself so return early
				return trimmed, "", ""
			} else {
				subject = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(trimmed), "subject:"))
			}
		} else if strings.HasPrefix(trimmedUpper, "CONTENT-TYPE:") && contentType == "" {
			contentType = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(trimmed), "content-type:"))
		}
	}
	return
}

// Look for shell statement and PIN/preset message match in the text part of an MIME mail. Return empty if no match.
func (sh *WebShell) findShellStatementInMIMEMail(contentType, subject, mailContent string) string {
	mimeMail := &mail.Message{
		Header: map[string][]string{"Content-Type": {contentType}},
		Body:   strings.NewReader(mailContent),
	}
	mediaType, params, err := mime.ParseMediaType(mimeMail.Header.Get("Content-Type"))
	if err == nil {
		if strings.HasPrefix(mediaType, "multipart/") {
			mr := multipart.NewReader(mimeMail.Body, params["boundary"])
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Printf("Mailsh failed to open MIME email '%s' - %v", subject, err)
					break
				}
				slurp, err := ioutil.ReadAll(p)
				if err != nil {
					log.Printf("Mailsh failed to read MIME email '%s' - %v", subject, err)
					break
				}
				partContentType := p.Header.Get("Content-Type")
				if strings.Contains(partContentType, "text") {
					if stmt := sh.matchPINInTextMailBoxy(subject, string(slurp)); stmt != "" {
						log.Printf("Mailsh has found statement in MIME email '%s'", subject)
						return stmt
					}
				}
			}
		}
	}
	log.Printf("Mailsh cannot find statement in MIME email '%s'", subject)
	return ""
}

// Look for PIN/preset message match in the Email text. Return empty if no match
func (sh *WebShell) matchPINInTextMailBoxy(subject, mailContent string) string {
	for _, line := range strings.Split(mailContent, "\n") {
		if stmt := sh.matchPresetOrPIN(line); stmt != "" {
			log.Printf("Mailsh has found statement in text email '%s'", subject)
			return stmt
		}
	}
	log.Printf("Mailsh cannot find statement in text email '%s'", subject)
	return ""
}

// Analyse the email as either MIME or plain text mail, whichever one yields PIN/preset message match, and run the statement in the mail body.
func (sh *WebShell) runStatementInEmail(subject, contentType, mailContent string) (stmt, output string) {
	stmt = sh.findShellStatementInMIMEMail(contentType, subject, mailContent)
	if stmt == "" {
		stmt = sh.matchPINInTextMailBoxy(subject, mailContent)
	}
	if stmt != "" {
		output = sh.runStatement(stmt, sh.MailTimeoutSec, sh.MailTruncateLen, false, false)
		log.Printf("Mailsh has run statement '%s' from email '%s'", stmt, subject)
	}
	return
}

// Read email message from stdin and process the statement in it.
func (sh *WebShell) processMail(mailContent string) {
	subject, contentType, replyTo := findSubjectAndReplyAddressInMail(mailContent)
	log.Printf("Mailsh is processing email '%s' and reply address is '%s'", subject, replyTo)
	if replyTo == "" {
		log.Printf("Mailsh failed to find reply address of email '%s'", subject)
		return
	}
	if sh.MysteriousAddr1 != "" && strings.HasSuffix(replyTo, sh.MysteriousAddr1) {
		log.Printf("Mailsh will respond to email '%s' in mysterious ways", subject)

		stmt, output := sh.runStatementInEmail(subject, contentType, mailContent)
		if stmt == "" {
			log.Printf("Mailsh failed to find statement to run in email '%s'", subject)
			return
		}
		sh.doMysteriousHTTPRequest(output)
	} else {
		// Match PIN/preset message in the mail body, run the statement and reply
		stmt, output := sh.runStatementInEmail(subject, contentType, mailContent)
		if stmt == "" {
			log.Printf("Mailsh failed to find statement to run in email '%s'", subject)
			return
		}
		log.Printf("Mailsh will email response for '%s' to %s", subject, replyTo)
		msg := fmt.Sprintf(EmailNotificationReplyFormat, stmt, output)
		if err := smtp.SendMail(sh.MailAgentAddressPort, nil, sh.MailFrom, []string{replyTo}, []byte(msg)); err != nil {
			log.Printf("Mailsh failed to send email response for '%s' to %s - %v", subject, replyTo, err)
		}
	}
}

// The HTTP message endpoint looks for statement to execute from request. The input/output conforms to Twilio SMS web hook.
func (sh *WebShell) httpMessageEndpoint(w http.ResponseWriter, r *http.Request) {
	if stmt := sh.matchPresetOrPIN(r.FormValue("Body")); stmt == "" {
		// No match, don't give much clue to the client though.
		http.Error(w, "404 page not found", http.StatusNotFound)
	} else {
		respOut := sh.runStatement(stmt, sh.WebTimeoutSec, sh.WebTruncateLen, true, true)
		writeHTTPResponse(w, respOut)
	}
}

// The HTTP Voice mark-up endpoint returns TwiML voice script.
func (sh *WebShell) httpVoiceMLEndpoint(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Got a call ML")
	w.Header().Set("Content-Type", "text/xml")
	w.Header().Set("Cache-Control", "must-revalidate")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="/%s" method="GET">
        <Say>
            Hello there
        </Say>
    </Gather>
    <Say>We are done ~!@#$%%&^^*()_+{}:"|?,./;'\][=-09</Say>
</Response>
`, sh.VoiceProcEndpoint)))
}

// The HTTP voice processing endpoint reads DTMF (input from Twilio request) and translates it into a statement and execute.
func (sh *WebShell) httpVoiceProcEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml")
	w.Header().Set("Cache-Control", "must-revalidate")
	w.WriteHeader(http.StatusOK)
	digits := r.FormValue("Digits")
	fmt.Printf("Digits are: '%s'\n", digits)
	w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
	<Say>You have entered %s</Say>
</Response>
`, digits)))
}

// Run HTTP server and block until the process exits.
func (sh *WebShell) runHTTPServer() {
	http.HandleFunc("/"+sh.MessageEndpoint, sh.httpMessageEndpoint)
	http.HandleFunc("/"+sh.VoiceMLEndpoint+".xml", sh.httpVoiceMLEndpoint)
	http.HandleFunc("/"+sh.VoiceProcEndpoint, sh.httpVoiceProcEndpoint)
	if err := http.ListenAndServeTLS(":"+strconv.Itoa(sh.ServerPort), sh.TLSCert, sh.TLSKey, nil); err != nil {
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
		flag.PrintDefaults()
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

	if websh.PIN == "" ||
		(mailMode && (websh.MailTimeoutSec < 1 || websh.MailTruncateLen < 1 || websh.MailFrom == "" || websh.MailAgentAddressPort == "")) ||
		(!mailMode && (websh.MessageEndpoint == "" || websh.ServerPort < 1 || websh.TLSCert == "" || websh.TLSKey == "" ||
			websh.WebTimeoutSec < 1 || websh.WebTruncateLen < 1)) {
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
			log.Printf("Websh will send Email notifications to %v", websh.MailRecipients)
		} else {
			log.Print("Websh will not send Email notifications")
		}
		websh.runHTTPServer()
	}
}
