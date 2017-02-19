// An Internet function or system function that takes a text command as input and responds with string text.
package feature

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	COMBINED_TEXT_SEP = "|" // Separate error and command output in the combined output
)

var (
	ErrExecTimeout      = errors.New("Timeout")
	ErrEmptyCommand     = errors.New("Empty command")
	ErrIncompleteConfig = errors.New("Incomplete configuration")
)

// Execution details for invoking a feature.
type Command struct {
	TimeoutSec int
	Content    string
}

// Modify command content to remove leading and trailing white spaces. If content becomes empty afterwards, return an error result.
func (cmd *Command) Trim() *Result {
	cmd.Content = strings.TrimSpace(cmd.Content)
	if cmd.Content == "" {
		return &Result{Error: ErrEmptyCommand, Output: ""}
	}
	return nil
}

func (cmd *Command) Lines() []string {
	return strings.Split(cmd.Content, "\n")
}

// Represent a useful feature that is capable of execution and provide execution result as feedback.
type Feature interface {
	IsConfigured() bool      // Return true only if configuration is present, this is called prior to Initialise().
	SelfTest() error         // Validate and test configuration.
	Initialise() error       // Prepare internal states.
	TriggerPrefix() string   // Return a command prefix string (e.g. ".t") to trigger the feature.
	Execute(Command) *Result // Execute the command and return result.
}

// Feedback from feature execution that gives human readable output and error (if any).
type Result struct {
	Error  error  // Result error if there is any
	Output string // Human readable normal output excluding error text
}

// Return error text or empty string if error is absent.
func (result *Result) ErrText() string {
	if result.Error == nil {
		return ""
	}
	return result.Error.Error()
}

// Return combined error text and output text
func (result *Result) CombinedText() (ret string) {
	if result.Error != nil {
		ret = result.Error.Error()
		if result.Output != "" {
			ret += COMBINED_TEXT_SEP
		}
	}
	ret += result.Output
	return
}

// Log a command prior to its execution.
func LogBeforeExecute(cmd Command) {
	log.Printf(`Will use up to %d seconds to run command: %s`, cmd.TimeoutSec, cmd.Content)
}

// Log a command after its execution. To log the result properly, call the function this way: defer func() {Log(a,b)}
func LogAfterExecute(cmd Command, result *Result) {
	var resultMsg string
	if result == nil {
		resultMsg = "(nil)"
	} else {
		resultMsg = result.CombinedText()
	}
	log.Printf(`Command "%s" has completed with result: %s`, cmd.Content, resultMsg)
}

// Send an HTTP request and return its response. Placeholders in URL template must be "%s".
func DoHTTP(timeoutSec int, method string, contentType string, reqBody io.Reader, reqFun func(*http.Request) error, urlTemplate string, urlValues ...interface{}) (
	status int, respBody []byte, err error) {
	// Encode values in URL path
	encodedURLValues := make([]interface{}, len(urlValues))
	for i, val := range urlValues {
		encodedURLValues[i] = url.QueryEscape(fmt.Sprint(val))
	}
	req, err := http.NewRequest(method, fmt.Sprintf(urlTemplate, encodedURLValues...), reqBody)
	if err != nil {
		return
	}
	// Allow function to further manipulate HTTP request
	if reqFun != nil {
		if err = reqFun(req); err != nil {
			return
		}
	}
	// Always carry Content-Type header
	if contentType == "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	} else {
		req.Header.Set("Content-Type", contentType)
	}
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return
	}
	defer response.Body.Close()
	status = response.StatusCode
	respBody, err = ioutil.ReadAll(response.Body)
	return
}

// If HTTP response constitutes any sort of error, return the error in a Result. Otherwise return nil.
func HTTPResponseError(status int, respBody []byte, err error) *Result {
	if err != nil {
		return &Result{Error: err, Output: string(respBody)}
	} else if status/200 != 1 {
		return &Result{Error: fmt.Errorf("HTTP %d: %s", status, string(respBody)), Output: string(respBody)}
	}
	return nil
}
