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
	"unicode"
)

const (
	magicWolframAlpha    = "#w"              // Message prefix that triggers WolframAlpha query
	magicTwilioSendSMS   = "#s"              // Message prefix that triggers sending outbound text via Twilio
	magicTwilioVoiceCall = "#c"              // Message prefix that triggers outbound calling via Twilio
	magicTwitterGet      = "#tg"             // Message prefix that triggers reading twitter timeline
	magicTwitterPost     = "#tp"             // Message prefix that triggers tweet
	twilioParamError     = "Parameter error" // Response text from a bad twilio call/text request
	maxOutputLength      = 2048              // Ultimate maximum length of response text (override user's configuration)
	emptyOutputResponse  = "EMPTY OUTPUT"    // Response to make if a command did not produce any response at all
)

var consecutiveSpacesRegex = regexp.MustCompile("[[:space:]]+") // Match consecutive spaces in statement output
var phoneNumberRegex = regexp.MustCompile(`\+[0-9]+`)           // Match a phone number with a prefix + sign

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

// Concatenate command execution error (if any) and output together into a single string, and truncate it to fit into maximum output length.
func LintOutput(outErr error, outText string, maxOutLen int, squeezeIntoOneLine bool) (out string) {
	outLines := make([]string, 0, 8)
	if outErr != nil {
		for _, line := range strings.Split(fmt.Sprint(outErr), "\n") {
			outLines = append(outLines, RemoveNonAscii(strings.TrimSpace(line)))
		}
	}
	for _, line := range strings.Split(outText, "\n") {
		outLines = append(outLines, RemoveNonAscii(strings.TrimSpace(line)))
	}
	if squeezeIntoOneLine {
		out = strings.Join(outLines, ";")
		out = consecutiveSpacesRegex.ReplaceAllString(out, " ")
	} else {
		out = strings.Join(outLines, "\n")
	}
	// Truncate to user-specified length, or maximum of 2KB.
	if maxOutLen <= 0 || maxOutLen > maxOutputLength {
		maxOutLen = maxOutputLength
	}
	if len(out) > maxOutLen {
		out = out[0:maxOutLen]
	}
	if out == "" {
		return emptyOutputResponse
	}
	return strings.TrimSpace(out)
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
	Twilio       TwilioClient
	Twitter      TwitterClient
	WolframAlpha WolframAlphaClient
}

// Validate configuration, make sure they look good.
func (run *CommandRunner) CheckConfig() error {
	if run.TimeoutSec < 3 {
		return fmt.Errorf("TimeoutSec(%d) should be at least 3", run.TimeoutSec)
	} else if run.TruncateLen < 10 || run.TruncateLen > maxOutputLength {
		return fmt.Errorf("TruncateLen(%d) should be in between 10 and %d", run.TimeoutSec, maxOutputLength)
	} else if len(run.PIN) < 7 {
		return errors.New("PIN should be at least 7 characters long")
	}
	return nil
}

// Execute the input command with strict timeout guarantee, return command output.
func (run *CommandRunner) RunCommand(cmd string, squeezeIntoOneLine bool) string {
	cmd = strings.TrimSpace(cmd)
	if run.SubHashSlashForPipe {
		cmd = strings.Replace(cmd, "#/", "|", -1)
	}
	log.Printf("Processing command '%s'...", cmd)

	var output string
	var err error

	if strings.HasPrefix(cmd, magicWolframAlpha) {
		// Run WolframAlpha query
		output, err = run.WolframAlpha.InvokeAPI(run.TimeoutSec, cmd[len(magicWolframAlpha):])
	} else if strings.HasPrefix(cmd, magicTwilioVoiceCall) {
		// The first phone number from input message is the to-number, extract and remove it before calling.
		inMessage := strings.TrimSpace(cmd[len(magicTwilioVoiceCall):])
		toNumber := phoneNumberRegex.FindString(inMessage)
		if toNumber == "" {
			output = twilioParamError
			goto out
		}
		inMessage = strings.Replace(inMessage, toNumber, "", 1)
		if err = run.Twilio.MakeCall(run.TimeoutSec, toNumber, inMessage); err == nil {
			output = "OK " + toNumber
		}
	} else if strings.HasPrefix(cmd, magicTwilioSendSMS) {
		// The first phone number from input message is the to-number, extract and remove it before texting.
		inMessage := strings.TrimSpace(cmd[len(magicTwilioVoiceCall):])
		toNumber := phoneNumberRegex.FindString(inMessage)
		if toNumber == "" {
			output = twilioParamError
			goto out
		}
		inMessage = strings.Replace(inMessage, toNumber, "", 1)
		if err = run.Twilio.SendText(run.TimeoutSec, toNumber, inMessage); err == nil {
			output = "OK " + toNumber
		}
	} else if strings.HasPrefix(cmd, magicTwitterGet) {
		// Read latest tweets from twitter time-line
		inParams := strings.TrimSpace(cmd[len(magicTwitterGet):])
		params := consecutiveSpacesRegex.Split(inParams, -1)
		var skip, count int
		skip, err = strconv.Atoi(params[0])
		if err != nil {
			goto out
		}
		count, err = strconv.Atoi(params[1])
		if err != nil {
			goto out
		}
		var tweets []Tweet
		tweets, err = run.Twitter.RetrieveLatest(run.TimeoutSec, skip, count)
		if err != nil {
			goto out
		}
		for _, tweet := range tweets {
			output += fmt.Sprintf("%s %s\n", tweet.User.Name, tweet.Text)
		}
	} else if strings.HasPrefix(cmd, magicTwitterPost) {
		// Post update to twitter time-line
		twitMessage := strings.TrimSpace(cmd[len(magicTwitterPost):])
		if err = run.Twitter.PostUpdate(run.TimeoutSec, twitMessage); err == nil {
			output = fmt.Sprintf("OK %d", len(twitMessage))
		}
	} else {
		// Run shell command
		var cmdOut []byte
		cmdOut, err = exec.Command("/usr/bin/timeout", "--preserve-status", strconv.Itoa(run.TimeoutSec), "/bin/bash", "-c", cmd).CombinedOutput()
		output = string(cmdOut)
	}

out:
	funcOutput := LintOutput(err, output, run.TruncateLen, squeezeIntoOneLine)
	log.Printf("Command '%s' finished with output - %s", cmd, funcOutput)
	run.Mailer.SendNotification(cmd, output)
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
