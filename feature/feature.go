package feature

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

const (
	COMBINED_TEXT_SEP = "|" // Separate error and command output in the combined output
)

var (
	ErrExecTimeout  = errors.New("Execution timeout")
	ErrEmptyCommand = errors.New("Command is empty")
)

// Represent a useful feature that is capable of execution and provide execution result as feedback.
type Feature interface {
	InitAndTest() error                        // Prepare internal states by running configuration and tests
	TriggerPrefix() string                     // Command prefix string to trigger the feature
	Execute(timeoutSec int, cmd string) Result // Feature execution and return the result
}

// Feedback from command execution that has human readable output and error - if any.
type Result interface {
	Err() error           // Execution error if there is any
	ErrText() string      // Human readable error text
	OutText() string      // Human readable normal output excluding error text
	CombinedText() string // Combined normal and error text
}

// Send an HTTP request and return its response.
func DoHTTP(timeoutSec int, method string, contentType string, reqBody io.Reader, urlTemplate string, urlValues ...interface{}) (
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
