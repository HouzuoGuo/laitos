// An Internet function or system function that takes a text command as input and responds with string text.
package feature

import (
	"errors"
	"github.com/HouzuoGuo/websh/httpclient"
	"log"
	"strings"
)

const (
	COMBINED_TEXT_SEP     = "|" // Separate error and command output in the combined output
	HTTP_TEST_TIMEOUT_SEC = 60  // Timeout for HTTP requests among those involved in self tests
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

type Trigger string // Trigger is a prefix string that is matched against command input to trigger a feature, each feature has a unique trigger.

// Represent a useful feature that is capable of execution and provide execution result as feedback.
type Feature interface {
	IsConfigured() bool      // Return true only if configuration is present, this is called prior to Initialise().
	SelfTest() error         // Validate and test configuration.
	Initialise() error       // Prepare internal states.
	Trigger() Trigger        // Return a prefix string that is matched against command input to trigger a feature, each feature has a unique trigger.
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

// If HTTP status is not 2xx or HTTP response already has an error, return an error result. Otherwise return nil.
func HTTPErrorToResult(resp httpclient.Response, err error) *Result {
	if err != nil {
		return &Result{Error: err, Output: string(resp.Body)}
	} else if respErr := resp.Non2xxToError(); respErr != nil {
		return &Result{Error: respErr, Output: string(resp.Body)}
	}
	return nil
}
