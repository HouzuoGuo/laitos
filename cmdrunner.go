package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	magicTwilioVoiceCall = "#c"           // Message prefix that triggers outbound calling via Twilio
	magicFacebookPost    = "#f"           // Message prefix that triggers Facebook posting
	magicTwilioSendSMS   = "#s"           // Message prefix that triggers sending outbound text via Twilio
	magicSeekOutput      = "#o"           // Message prefix that cuts command output by position and overrides length restriction
	magicTwitterGet      = "#tg"          // Message prefix that triggers reading twitter timeline
	magicTwitterPost     = "#tp"          // Message prefix that triggers tweet
	magicWolframAlpha    = "#w"           // Message prefix that triggers WolframAlpha query
	maxOutputLen         = 2048           // Ultimate maximum length of response text (override user's configuration)
	emptyOutputResponse  = "EMPTY OUTPUT" // Response to make if a command did not produce any response at all
)

var consecutiveSpacesRegex = regexp.MustCompile("[[:space:]]+") // Match consecutive spaces in statement output
var phoneNumberRegex = regexp.MustCompile(`\+[0-9]+`)           // Match a phone number with a prefix + sign
var paramErr = errors.New("BadParam")                           // Error response in case of bad parameter input
var timeoutErr = errors.New("ProcTimeout")                      // Error response in cae of shell process timeout

// Remove non-ASCII sequences form the input string and return.
func RemoveNonAscii(in string) string {
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

// Compact optional error message together with command's normal output.
func LintOutput(err error, text string, seekPos, outputLen int, squeezeIntoOneLine bool) string {
	var errText, outText string
	errLines := make([]string, 0, 8)
	outLines := make([]string, 0, 8)
	// Lint characters from error message
	if err != nil {
		for _, line := range strings.Split(err.Error(), "\n") {
			errLines = append(errLines, RemoveNonAscii(strings.TrimSpace(line)))
		}
	}
	// Compact error message
	if squeezeIntoOneLine {
		errText = strings.Join(errLines, ";")
		errText = consecutiveSpacesRegex.ReplaceAllString(errText, " ")
	} else {
		errText = strings.Join(errLines, "\n")
	}
	// Lint characters from output text
	for _, line := range strings.Split(text, "\n") {
		outLines = append(outLines, RemoveNonAscii(strings.TrimSpace(line)))
	}
	// Compact normal output
	if squeezeIntoOneLine {
		outText = strings.Join(outLines, ";")
		outText = consecutiveSpacesRegex.ReplaceAllString(outText, " ")
	} else {
		outText = strings.Join(outLines, "\n")
	}
	// Apply seek position to output only
	if seekPos < 0 || seekPos >= len(outText)-1 {
		seekPos = 0
	}
	outText = outText[seekPos:]
	// Apply truncation to the entire output
	if outputLen <= 0 || outputLen > maxOutputLen {
		outputLen = maxOutputLen
	}
	// Join error and nromal text
	ret := errText
	if ret != "" {
		if squeezeIntoOneLine {
			ret += ";"
		} else {
			ret += "\n"
		}
	}
	ret += outText
	// Cut out at the maximum output length
	endIndex := outputLen
	if endIndex > len(ret) {
		endIndex = len(ret)
	}
	ret = ret[:endIndex]
	if ret == "" {
		return emptyOutputResponse
	}
	return strings.TrimSpace(ret)
}

// Run shell command or another supported action with timeout and output length constraints.
type CommandRunner struct {
	SubHashSlashForPipe bool              // Substitute char sequence #/ from incoming command for char | before command execution
	SqueezeIntoOneLine  bool              // Squeeze output lines into a single line.
	TimeoutSec          int               // In-progress commands are killed after this number of seconds
	TruncateLen         int               // Truncate output to this length before responding back to the user
	PIN                 string            // Mandatory prefix in each request command to authorise execution
	PresetMessages      map[string]string // Pre-defined mapping of secret phrases and their  corresponding command

	Mailer       Mailer
	Facebook     FacebookClient
	Twilio       TwilioClient
	Twitter      TwitterClient
	WolframAlpha WolframAlphaClient
}

// Validate configuration, make sure they look good.
func (run *CommandRunner) CheckConfig() error {
	if run.TimeoutSec < 3 {
		return fmt.Errorf("TimeoutSec(%d) should be at least 3", run.TimeoutSec)
	} else if run.TruncateLen < 10 || run.TruncateLen > maxOutputLen {
		return fmt.Errorf("TruncateLen(%d) should be in between 10 and %d", run.TimeoutSec, maxOutputLen)
	} else if len(run.PIN) < 7 {
		return errors.New("PIN should be at least 7 characters long")
	}
	return nil
}

// Execute the input command with strict timeout guarantee, return command output.
func (run *CommandRunner) RunCommand(cmd string, squeezeIntoOneLine bool) string {
	log.Printf("Processing command '%s'...", cmd)

	var cmdOut string
	var cmdErr error
	var seekPos int
	var outputLen = run.TruncateLen
	cmd = strings.TrimSpace(cmd)
	if len(cmd) == 0 {
		cmdErr = paramErr
		goto out
	}

	if run.SubHashSlashForPipe {
		cmd = strings.Replace(cmd, "#/", "|", -1)
	}

	// Match the special prefix that picks position in the output
	// It looks like: "prefix skip_num len"
	if strings.HasPrefix(cmd, magicSeekOutput) {
		if len(cmd) == len(magicSeekOutput) {
			cmdErr = paramErr
			goto out
		}

		// Parse the next two numeric parameters
		cmd = strings.TrimSpace(cmd[len(magicSeekOutput):])
		params := consecutiveSpacesRegex.Split(cmd, 3)
		if len(params) != 3 {
			cmdErr = paramErr
			goto out
		}
		var convErr error
		seekPos, convErr = strconv.Atoi(params[0])
		if convErr != nil {
			cmdErr = paramErr
			goto out
		}
		outputLen, convErr = strconv.Atoi(params[1])
		if convErr != nil {
			cmdErr = paramErr
			goto out
		}
		cmd = strings.TrimSpace(params[2])
	}
	// Match special trigger prefixes
	if strings.HasPrefix(cmd, magicWolframAlpha) {
		// Run WolframAlpha query
		query := strings.TrimSpace(cmd[len(magicWolframAlpha):])
		if len(query) == 0 {
			cmdErr = paramErr
			goto out
		}
		cmdOut, cmdErr = run.WolframAlpha.InvokeAPI(run.TimeoutSec, query)
	} else if strings.HasPrefix(cmd, magicTwilioVoiceCall) {
		// The first phone number from input message is the to-number, extract and remove it before calling.
		inMessage := strings.TrimSpace(cmd[len(magicTwilioVoiceCall):])
		toNumber := phoneNumberRegex.FindString(inMessage)
		if len(toNumber) == 0 {
			cmdErr = paramErr
			goto out
		}
		inMessage = strings.Replace(inMessage, toNumber, "", 1)
		if len(inMessage) == 0 {
			cmdErr = paramErr
			goto out
		}
		if cmdErr = run.Twilio.MakeCall(run.TimeoutSec, toNumber, inMessage); cmdErr == nil {
			cmdOut = "OK " + toNumber
		}
	} else if strings.HasPrefix(cmd, magicTwilioSendSMS) {
		// The first phone number from input message is the to-number, extract and remove it before texting.
		inMessage := strings.TrimSpace(cmd[len(magicTwilioVoiceCall):])
		toNumber := phoneNumberRegex.FindString(inMessage)
		if len(toNumber) == 0 {
			cmdErr = paramErr
			goto out
		}
		inMessage = strings.Replace(inMessage, toNumber, "", 1)
		if len(inMessage) == 0 {
			cmdErr = paramErr
			goto out
		}
		if cmdErr = run.Twilio.SendText(run.TimeoutSec, toNumber, inMessage); cmdErr == nil {
			cmdOut = "OK " + toNumber
		}
	} else if strings.HasPrefix(cmd, magicTwitterGet) {
		// Read latest tweets from twitter time-line
		inParams := strings.TrimSpace(cmd[len(magicTwitterGet):])
		params := consecutiveSpacesRegex.Split(inParams, -1)
		if len(params) < 2 {
			cmdErr = paramErr
			goto out
		}
		skip, convErr := strconv.Atoi(params[0])
		if convErr != nil {
			cmdErr = paramErr
			goto out
		}
		count, convErr := strconv.Atoi(params[1])
		if convErr != nil {
			cmdErr = paramErr
			goto out
		}
		var tweets []Tweet
		tweets, cmdErr = run.Twitter.RetrieveLatest(run.TimeoutSec, skip, count)
		if cmdErr != nil {
			goto out
		}
		for _, tweet := range tweets {
			cmdOut += fmt.Sprintf("%s %s\n", tweet.User.Name, tweet.Text)
		}
	} else if strings.HasPrefix(cmd, magicTwitterPost) {
		// Post update to twitter time-line
		twitMessage := strings.TrimSpace(cmd[len(magicTwitterPost):])
		if len(twitMessage) == 0 {
			cmdErr = paramErr
			goto out
		}
		if cmdErr = run.Twitter.Tweet(run.TimeoutSec, twitMessage); cmdErr == nil {
			cmdOut = fmt.Sprintf("OK %d", len(twitMessage))
		}
	} else if strings.HasPrefix(cmd, magicFacebookPost) {
		// Post status update to Facebook
		fbStatus := strings.TrimSpace(cmd[len(magicFacebookPost):])
		if len(fbStatus) == 0 {
			cmdErr = paramErr
			goto out
		}
		if cmdErr = run.Facebook.WriteStatus(run.TimeoutSec, fbStatus); cmdErr == nil {
			cmdOut = fmt.Sprintf("OK %d", len(fbStatus))
		}
	} else {
		if len(cmd) == 0 {
			cmdErr = paramErr
			goto out
		}
		outChan := make(chan []byte, 1)
		errChan := make(chan error, 1)
		// Run shell command and collect output combined.
		shellProc := exec.Command("/bin/bash", "-c", cmd)
		go func() {
			out, err := shellProc.CombinedOutput()
			outChan <- out
			errChan <- err
		}()
		// If timeout occurs, kill the process.
		select {
		case shellOut := <-outChan:
			cmdOut = string(shellOut)
			cmdErr = <-errChan
		case <-time.After(time.Duration(run.TimeoutSec) * time.Second):
			if shellProc.Process != nil {
				if cmdErr = shellProc.Process.Kill(); cmdErr == nil {
					cmdErr = timeoutErr
				}
			}
		}
	}

out:
	funcOutput := LintOutput(cmdErr, cmdOut, seekPos, outputLen, squeezeIntoOneLine)
	log.Printf("Command '%s' finished with output - %s", cmd, funcOutput)
	run.Mailer.SendNotification(cmd, cmdOut)
	return funcOutput
}

// Match an input line against preset message or PIN, return the command portion of the line. Return empty string if no match.
func (run *CommandRunner) FindCommand(inputLine string) string {
	if run.PIN == "" {
		// Safe guard against an empty PIN
		return ""
	}
	inputLine = strings.TrimSpace(inputLine)
	// Try matching against preset
	if run.PresetMessages != nil {
		for preset, cmd := range run.PresetMessages {
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
	if len(inputLine) > len(run.PIN) && inputLine[0:len(run.PIN)] == run.PIN {
		return strings.TrimSpace(inputLine[len(run.PIN):])
	}
	return ""
}
