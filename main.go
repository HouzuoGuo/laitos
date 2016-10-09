/*
A simple web server daemon enabling basic shell access via API calls.
Good for emergency system shutdown/reboot, and executing privileged/unprivileged shell code.

The program can run in two modes:
- HTTPS daemon mode, secured by endpoint port number + endpoint name + PIN.
- Mail processing mode (~/.forward), secured by your username + PIN.

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
	"math/rand"
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
	"unicode"
)

const (
	magicWebshMailSubject = "websh" // Magic keyword that appears in websh mail notifications to prevent recursion in mail processing
	magicWolframAlpha     = "#w"    // Message prefix that triggers WolframAlpha query
	magicTwilioSendSMS    = "#t"    // Message prefix that triggers sending outbound text via Twilio
	magicTwilioVoiceCall  = "#c"    // Message prefix that triggers outbound calling via Twilio
	twilioParamError      = "Error" // Response text from a bad twilio call/text request
)

var mailAddressRegex = regexp.MustCompile(`[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+`) // Match a mail address in header
var consecutiveSpacesRegex = regexp.MustCompile("[[:space:]]+")                             // Match consecutive spaces in statement output
var phoneNumberRegex = regexp.MustCompile(`\+[0-9]+`)
var mailNotificationReplyFormat = "Subject: " + magicWebshMailSubject + " - %s\r\n\r\n%s" // Subject and body format of notification and reply mails

// Remove non-ASCII sequences form the input string and return.
func removeNonAscii(in string) string {
	var out bytes.Buffer
	for _, r := range in {
		if unicode.IsPrint(r) {
			if r < 128 {
				out.WriteRune(r)
			} else {
				out.WriteRune(' ')
			}
		}
	}
	return out.String()
}

// Concatenate command execution error (if any) and output together into a single string, and truncate it to fit into maximum output length.
func lintCommandOutput(outErr error, outText string, maxOutLen int, squeezeIntoOneLine, truncateToLen bool) (out string) {
	outLines := make([]string, 0, 8)
	if outErr != nil {
		for _, line := range strings.Split(fmt.Sprint(outErr), "\n") {
			outLines = append(outLines, removeNonAscii(strings.TrimSpace(line)))
		}
	}
	for _, line := range strings.Split(outText, "\n") {
		outLines = append(outLines, removeNonAscii(strings.TrimSpace(line)))
	}
	if squeezeIntoOneLine {
		out = strings.Join(outLines, "#")
		out = consecutiveSpacesRegex.ReplaceAllString(out, " ")
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

type WebShell struct {
	MessageEndpoint     string // The secret API endpoint name for messaging in daemon mode
	VoiceMLEndpoint     string // The secret API endpoint name that serves TwiML voice script in daemon mode
	VoiceProcEndpoint   string // The secret API endpoint name that responds to TwiML voice script
	VoiceEndpointPrefix string // The HTTP scheme and/or host name and/or URL prefix that will correctly construct URLs leading to ML and Proc endpoints
	ServerPort          int    // The port HTTP server listens on in daemon mode
	PIN                 string // The pre-shared secret pin to enable command execution in both daemon and mail mode
	TLSCert             string // Location of HTTP TLS certificate in daemon mode
	TLSKey              string // Location of HTTP TLS key in daemon mode

	SubHashSlashForPipe bool // Substitute char sequence #/ from incoming command for char | before command execution
	WebTimeoutSec       int  // When reached from web API, WolframAlpha query/shell command is killed after this number of seconds.
	WebTruncateLen      int  // When reached from web API, truncate command execution result to this length.

	MailTimeoutSec       int      // When reached from mail API, WolframAlpha query/shell command is killed after this number of seconds.
	MailTruncateLen      int      // When reached from mail API, truncate command execution result to this length.
	MailRecipients       []string // List of mail addresses that receive notification after each command
	MailFrom             string   // FROM address of the mail notifications
	MailAgentAddressPort string   // Address and port number of mail transportation agent for sending notifications

	MysteriousURL         string   // intentionally undocumented
	MysteriousAddr1       string   // intentionally undocumented
	MysteriousAddr2       string   // intentionally undocumented
	MysteriousID1         string   // intentionally undocumented
	MysteriousID2         string   // intentionally undocumented
	MysteriousCmds        []string // intentionally undocumented
	MysteriousCmdIntvHour int      // intentionally undocumented

	TwilioNumber     string // Twilio telephone number for outbound call and SMS
	TwilioSID        string // Twilio account SID
	TwilioAuthSecret string // Twilio authentication secret token

	WolframAlphaAppID string // WolframAlpha application ID for consuming its APIs

	PresetMessages map[string]string // Pre-defined mapping of secret phrases and their  corresponding command
}

// Return true only if all mail parameters are present (hence, enabling mail notifications).
func (sh *WebShell) isMailNotificationEnabled() bool {
	return sh.MailAgentAddressPort != "" && sh.MailFrom != "" && len(sh.MailRecipients) > 0
}

// Log an executed command in standard error and send an mail notification if it is enabled.
func (sh *WebShell) logAndNotify(command, output string) {
	log.Printf("Websh has run '%s': %s", command, output)
	if sh.isMailNotificationEnabled() {
		go func() {
			msg := fmt.Sprintf(mailNotificationReplyFormat, command, output)
			if err := smtp.SendMail(sh.MailAgentAddressPort, nil, sh.MailFrom, sh.MailRecipients, []byte(msg)); err == nil {
				log.Printf("Websh has sent mail notifications for '%s' to %v", command, sh.MailRecipients)
			} else {
				log.Printf("Websh failed to send notification mail: %v", err)
			}
		}()
	}
}

/*
=======================================
WolframAlpha
=======================================
*/

// Extract "pods" from WolframAlpha API response in XML.
func (sh *WebShell) waExtractResponse(xmlBody []byte) string {
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
			outBuf.WriteRune(';')
		}
	}
	return outBuf.String()
}

// Call WolframAlpha API with the text query.
func (sh *WebShell) waCallAPI(timeoutSec int, query string) string {
	request, err := http.NewRequest(
		"GET",
		fmt.Sprintf("https://api.wolframalpha.com/v2/query?appid=%s&input=%s&format=plaintext", sh.WolframAlphaAppID, url.QueryEscape(query)),
		bytes.NewReader([]byte{}))
	if err != nil {
		log.Printf("Failed to initialise WolframAlpha HTTP request for '%s': %v", query, err)
		return ""
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	response, err := client.Do(request)
	if err != nil {
		log.Printf("Failed to make WolframAlpha request for '%s': %v", query, err)
		return ""
	}
	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	textResponse := sh.waExtractResponse(body)
	log.Printf("Got response from WolframAlpha for '%s': error %v, status %d, output %s", query, err, response.StatusCode, textResponse)
	return textResponse
}

/*
=======================================
Mysterious functions
=======================================
*/

// Return true only if mysterious command should run at regular interval.
func (sh *WebShell) isMysteriousCmdEnabled() bool {
	return len(sh.MysteriousCmds) != 0 && sh.MysteriousCmdIntvHour > 0 && sh.MysteriousURL != ""
}

// This mysterious HTTP call is intentionally undocumented hahahaha.
func (sh *WebShell) mysteriousCallAPI(rawMessage string) {
	requestBody := fmt.Sprintf("ReplyAddress=%s&ReplyMessage=%s&MessageId=%s&Guid=%s",
		url.QueryEscape(sh.MysteriousAddr2), url.QueryEscape(rawMessage), sh.MysteriousID1, sh.MysteriousID2)

	request, err := http.NewRequest("POST", sh.MysteriousURL, bytes.NewReader([]byte(requestBody)))
	if err != nil {
		log.Printf("MysteriousShell cannot initialise HTTP request for '%s': %v", rawMessage, err)
		return
	}
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/52.0.2743.116 Safari/537.36")
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

// Run the mysterious command at regular interval. Blocks caller.
func (sh *WebShell) mysteriousCmdAtInterval() {
	for counter := 0; ; counter++ {
		output := sh.cmdRun(sh.MysteriousCmds[counter%len(sh.MysteriousCmds)], sh.WebTimeoutSec, sh.WebTruncateLen, true, true)
		sh.mysteriousCallAPI(output)
		// Some random seconds of delay make it more mysterious
		time.Sleep(time.Duration(sh.MysteriousCmdIntvHour*3600+rand.Intn(300)) * time.Second)
	}
}

/*
=======================================
Shell/WolframAlpha/Twilio command execution
=======================================
*/

// Invoke Twilio account's HTTP endpoint, passing additional number and caller specified parameters. Return "OK" or "Error" string.
func (sh *WebShell) twilioInvokeAPI(timeoutSec int, finalEndpoint string, toNumber string, otherParams map[string]string) string {
	urlParams := url.Values{"From": []string{sh.TwilioNumber}, "To": []string{toNumber}}
	for key, val := range otherParams {
		urlParams[key] = []string{val}
	}
	fmt.Println("TWILIO POST PARAMS ARE", urlParams.Encode())
	request, err := http.NewRequest(
		"POST",
		fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/%s", sh.TwilioSID, finalEndpoint),
		strings.NewReader(urlParams.Encode()))
	if err != nil {
		log.Printf("Failed to initialise Twilio HTTP request for '%s': %v", finalEndpoint, err)
		return ""
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	request.SetBasicAuth(sh.TwilioSID, sh.TwilioAuthSecret)
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	response, err := client.Do(request)
	if err != nil {
		log.Printf("Failed to make Twilio request for '%s': %v", finalEndpoint, err)
		return ""
	}
	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	log.Printf("Got response from Twilio for: error %v, status %d, output %s", err, response.StatusCode, string(body))
	if response.StatusCode/200 == 1 {
		return "OK " + toNumber
	} else {
		return "Error " + toNumber
	}
}

// Execute the input command with strict timeout guarantee, return command output.
func (sh *WebShell) cmdRun(cmd string, timeoutSec, maxOutLen int, squeezeIntoOneLine, truncateToLen bool) (output string) {
	cmd = strings.TrimSpace(cmd)
	log.Printf("Processing command '%s'", cmd)
	if strings.HasPrefix(cmd, magicWolframAlpha) {
		output = lintCommandOutput(nil, sh.waCallAPI(timeoutSec, cmd[len(magicWolframAlpha):]), maxOutLen, squeezeIntoOneLine, truncateToLen)
	} else if strings.HasPrefix(cmd, magicTwilioVoiceCall) {
		// The first phone number from input message is the to-number, extract and remove it before calling.
		inMessage := cmd[len(magicTwilioVoiceCall):]
		toNumber := phoneNumberRegex.FindString(inMessage)
		if toNumber == "" {
			return twilioParamError
		}
		inMessage = strings.Replace(inMessage, toNumber, "", 1)
		return sh.twilioInvokeAPI(timeoutSec, "Calls.json", toNumber,
			map[string]string{"Url": "http://twimlets.com/message?Message=" + url.QueryEscape(inMessage+" repeat once more "+inMessage)})
	} else if strings.HasPrefix(cmd, magicTwilioSendSMS) {
		// The first phone number from input message is the to-number, extract and remove it before texting.
		inMessage := cmd[len(magicTwilioVoiceCall):]
		toNumber := phoneNumberRegex.FindString(inMessage)
		if toNumber == "" {
			return twilioParamError
		}
		inMessage = strings.Replace(inMessage, toNumber, "", 1)
		return sh.twilioInvokeAPI(timeoutSec, "Messages.json", toNumber, map[string]string{"Body": inMessage})
	} else {
		if sh.SubHashSlashForPipe {
			cmd = strings.Replace(cmd, "#/", "|", -1)
		}
		outBytes, status := exec.Command("/usr/bin/timeout", "--preserve-status", strconv.Itoa(timeoutSec), "/bin/bash", "-c", cmd).CombinedOutput()
		output = lintCommandOutput(status, string(outBytes), maxOutLen, squeezeIntoOneLine, truncateToLen)
	}
	sh.logAndNotify(cmd, output)
	return
}

// Match an input line against preset message or PIN, return the command portion of the line. Return empty string if no match.
func (sh *WebShell) cmdFind(inputLine string) string {
	if sh.PIN == "" {
		// Safe guard against an empty PIN
		return ""
	}
	inputLine = strings.TrimSpace(inputLine)
	// Try matching against preset
	if sh.PresetMessages != nil {
		for preset, cmd := range sh.PresetMessages {
			if preset == "" || cmd == "" {
				// Safe guard against an empty preset message or empty command
				return ""
			}
			if len(inputLine) < len(preset) {
				continue
			}
			if inputLine[0:len(preset)] == preset {
				return cmd
			}
		}
	}
	// Try matching against PIN, the use of > is intentional to enforce minimum length of 1 character in the command.
	if len(inputLine) > len(sh.PIN) && inputLine[0:len(sh.PIN)] == sh.PIN {
		return strings.TrimSpace(inputLine[len(sh.PIN):])
	}
	return ""
}

/*
=======================================
Process plain text and multi-part mails
=======================================
*/

// Look for a reply address in the mail text (reply-to or from). Return empty string if such address is not found.
func mailGetProperties(mailContent string) (subject string, contentType string, replyTo string) {
	for _, line := range strings.Split(mailContent, "\n") {
		trimmed := strings.TrimSpace(line)
		trimmedUpper := strings.ToUpper(trimmed)
		if strings.HasPrefix(trimmedUpper, "FROM:") && replyTo == "" {
			if address := mailAddressRegex.FindString(trimmed); address != "" {
				replyTo = address
			}
		} else if strings.HasPrefix(trimmedUpper, "REPLY-TO:") {
			// Reply-to is preferred over From
			if address := mailAddressRegex.FindString(trimmed); address != "" {
				replyTo = address
			}
		} else if strings.HasPrefix(trimmedUpper, "SUBJECT:") {
			if strings.Contains(trimmedUpper, strings.ToUpper(magicWebshMailSubject)) {
				// Avoid recurse on mails sent by websh itself so return early
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

// Process an entire mail message and run the command within.
func (sh *WebShell) mailProcess(mailContent string) {
	subject, contentType, replyTo := mailGetProperties(mailContent)
	if replyTo == "" {
		log.Printf("Mailsh failed to find reply address of mail '%s'", subject)
		return
	}
	log.Printf("Mailsh is processing mail '%s' (type %s, reply to %s)", subject, contentType, replyTo)
	if sh.MysteriousAddr1 != "" && strings.HasSuffix(replyTo, sh.MysteriousAddr1) {
		log.Printf("Mailsh will respond to mail '%s' in undocumented ways", subject)
		cmd, output := sh.mailRunCmd(subject, contentType, mailContent)
		if cmd == "" {
			return
		}
		sh.mysteriousCallAPI(output)
	} else {
		// Match PIN/preset message in the mail body, run the command and reply
		cmd, output := sh.mailRunCmd(subject, contentType, mailContent)
		if cmd == "" {
			return
		}
		msg := fmt.Sprintf(mailNotificationReplyFormat, cmd, output)
		if err := smtp.SendMail(sh.MailAgentAddressPort, nil, sh.MailFrom, []string{replyTo}, []byte(msg)); err != nil {
			log.Printf("Mailsh failed to respond to '%s' %s - %v", subject, replyTo, err)
		}
	}
}

// Find and run command from multipart or plain text mail content.
func (sh *WebShell) mailRunCmd(subject, contentType, mailContent string) (cmd, output string) {
	cmd = sh.mailFindCmdMultipart(contentType, subject, mailContent)
	if cmd == "" {
		cmd = sh.mailFindCmdPlainText(subject, mailContent)
	}
	if cmd != "" {
		output = sh.cmdRun(cmd, sh.MailTimeoutSec, sh.MailTruncateLen, false, false)
	}
	return
}

// Look for PIN/preset message match in the mail text (no multipart). Return empty if no match
func (sh *WebShell) mailFindCmdPlainText(subject, mailContent string) string {
	for _, line := range strings.Split(mailContent, "\n") {
		if cmd := sh.cmdFind(line); cmd != "" {
			log.Printf("Mailsh found command in '%s'", subject)
			return cmd
		}
	}
	log.Printf("Mailsh cannot find command in '%s'", subject)
	return ""
}

// Look for command or preset message match in the text part of a mutlipart mail. Return empty if no match.
func (sh *WebShell) mailFindCmdMultipart(contentType, subject, mailContent string) string {
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
					log.Printf("Mailsh failed to open multipart mail '%s' - %v", subject, err)
					break
				}
				slurp, err := ioutil.ReadAll(p)
				if err != nil {
					log.Printf("Mailsh failed to read multipart mail '%s' - %v", subject, err)
					break
				}
				partContentType := p.Header.Get("Content-Type")
				if strings.Contains(partContentType, "text") {
					if cmd := sh.mailFindCmdPlainText(subject, string(slurp)); cmd != "" {
						log.Printf("Mailsh has found command in multipart mail '%s'", subject)
						return cmd
					}
				}
			}
		}
	}
	log.Printf("Mailsh cannot find command in multipart mail '%s'", subject)
	return ""
}

/*
=======================================
Voice pocessing functions
=======================================
*/
var dtmfDecode = map[string]string{
	` `:   ` `,
	`111`: `!`, `112`: `@`, `113`: `#`, `114`: `$`, `115`: `%`, `116`: `^`, `117`: `&`, `118`: `*`, `119`: `(`,
	`121`: "`", `122`: `~`, `123`: `)`, `124`: `-`, `125`: `_`, `126`: `=`, `127`: `+`, `128`: `[`, `129`: `{`,
	`131`: `]`, `132`: `}`, `133`: `\`, `134`: `|`, `135`: `;`, `136`: `:`, `137`: `'`, `138`: `"`, `139`: `,`,
	`141`: `<`, `142`: `.`, `143`: `>`, `144`: `/`, `145`: `?`,
	`1`: `0`, `11`: `1`, `12`: `2`, `13`: `3`, `14`: `4`, `15`: `5`, `16`: `6`, `17`: `7`, `18`: `8`, `19`: `9`,
	`2`: "a", `22`: `b`, `222`: `c`,
	`3`: "d", `33`: `e`, `333`: `f`,
	`4`: "g", `44`: `h`, `444`: `i`,
	`5`: "j", `55`: `k`, `555`: `l`,
	`6`: "m", `66`: `n`, `666`: `o`,
	`7`: "p", `77`: `q`, `777`: `r`, `7777`: `s`,
	`8`: "t", `88`: `u`, `888`: `v`,
	`9`: "w", `99`: `x`, `999`: `y`, `9999`: `z`,
} // Decode sequence of DTMF digits into letters and symbols

// Decode a sequence of character string sent via DTMF. Input is a sequence of key names (0-9 and *)
func voiceDecodeDTMF(digits string) string {
	digits = strings.TrimSpace(digits)
	if len(digits) == 0 {
		return ""
	}
	// Break input into consecutive digits and asterisks
	letters := make([]string, 0, 64)
	var accumulator bytes.Buffer
	for _, char := range digits {
		switch char {
		case ' ':
			fallthrough
		case '\n':
			fallthrough
		case '\r':
			fallthrough
		case '\t':
			// Skip spaces
			continue
		case '*': // shift
			if accumulator.Len() > 0 {
				letters = append(letters, accumulator.String())
				accumulator.Reset()
			}
			letters = append(letters, "*")
		case '0': // type a single space, or mark ending of a digit sequence
			if accumulator.Len() == 0 {
				letters = append(letters, " ")
			} else {
				letters = append(letters, accumulator.String())
				accumulator.Reset()
			}
		default: // digit sequence
			accumulator.WriteRune(char)
		}
	}
	if accumulator.Len() > 0 {
		letters = append(letters, accumulator.String())
	}
	// Translate digit sequences into character string
	var message bytes.Buffer
	var shift bool
	for _, charseq := range letters {
		if charseq == "*" {
			shift = !shift
		} else {
			decoded, found := dtmfDecode[charseq]
			if !found {
				log.Printf("DTMF decoding table cannot decode '%s'", charseq)
				continue
			}
			if shift {
				decoded = strings.ToUpper(decoded)
			}
			message.WriteString(decoded)
		}
	}
	return message.String()
}

/*
=======================================
HTTP API functions
=======================================
*/

// Run HTTP server and block until the process exits.
func (sh *WebShell) httpRunServer() {
	http.HandleFunc("/"+sh.MessageEndpoint, sh.httpAPIMessage)
	http.HandleFunc("/"+sh.VoiceMLEndpoint, sh.httpAPIVoiceGreeting)
	http.HandleFunc("/"+sh.VoiceProcEndpoint, sh.httpAPIVoiceMessage)
	if err := http.ListenAndServeTLS(":"+strconv.Itoa(sh.ServerPort), sh.TLSCert, sh.TLSKey, nil); err != nil {
		log.Panicf("Failed to start HTTPS server - %v", err)
	}
}

// Look for command to execute from request body. The request/response content conform to Twilio SMS hook requirements.
func (sh *WebShell) httpAPIMessage(w http.ResponseWriter, r *http.Request) {
	if cmd := sh.cmdFind(r.FormValue("Body")); cmd == "" {
		// No match, don't give much clue to the client though.
		http.Error(w, "404 page not found", http.StatusNotFound)
	} else {
		output := sh.cmdRun(cmd, sh.WebTimeoutSec, sh.WebTruncateLen, true, true)
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
func (sh *WebShell) httpAPIVoiceGreeting(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml")
	w.Header().Set("Cache-Control", "must-revalidate")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s%s" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say voice="man" loop="1" language="en">Hello</Say>
    </Gather>
</Response>
`, sh.VoiceEndpointPrefix, sh.VoiceProcEndpoint)))
}

// The HTTP voice processing endpoint reads DTMF (input from Twilio request) and translates it into a statement and execute.
func (sh *WebShell) httpAPIVoiceMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml")
	w.Header().Set("Cache-Control", "must-revalidate")
	w.WriteHeader(http.StatusOK)
	digits := r.FormValue("Digits")
	decodedLetters := voiceDecodeDTMF(digits)
	log.Printf("Voice message got digits: %s (decoded - %s)", digits, decodedLetters)
	if cmd := sh.cmdFind(decodedLetters); cmd == "" {
		// PIN mismatch
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
	<Say>Sorry</Say>
	<Hangup/>
</Response>
`))
	} else {
		// Speak the command result and repeat
		output := sh.cmdRun(cmd, sh.WebTimeoutSec, sh.WebTruncateLen, true, true)
		var escapeOutput bytes.Buffer
		if err := xml.EscapeText(&escapeOutput, []byte(output)); err != nil {
			log.Printf("XML escape failed - %v", err)
		}
		w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s%s" method="POST" timeout="15" finishOnKey="#" numDigits="1000">
        <Say voice="man" loop="1" language="en">%s over</Say>
    </Gather>
</Response>
`, sh.VoiceEndpointPrefix, sh.VoiceProcEndpoint, escapeOutput.String())))
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())
	// Read configuration file path from CLI parameter
	var configFilePath string
	var mailMode bool
	flag.StringVar(&configFilePath, "configfilepath", "", "Path to the configuration file")
	flag.BoolVar(&mailMode, "mailmode", false, "True if the program is processing an incoming mail, false if the program is running as a daemon")
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
		websh.mailProcess(string(mailContent))
		return
	} else {
		if websh.isMailNotificationEnabled() {
			log.Printf("Will send mail notifications to %v", websh.MailRecipients)
		} else {
			log.Print("Will not send mail notifications")
		}
		if websh.isMysteriousCmdEnabled() {
			log.Print("Will run mysterious command in background at regular interval")
			go websh.mysteriousCmdAtInterval()
		} else {
			log.Print("Will not run mysterious command in background")
		}
		websh.httpRunServer() // blocks
	}
}
