package feature

import (
	"bytes"
	"encoding/xml"
	"errors"
	"github.com/HouzuoGuo/laitos/httpclient"
	"strings"
)

// Send query to WolframAlpha.
type WolframAlpha struct {
	AppID string `json:"AppID"` // WolframAlpha API AppID ("Developer Portal - My Apps - <name> - AppID")
}

var TestWolframAlpha = WolframAlpha{} // AppID is set by init_test.go

func (wa *WolframAlpha) IsConfigured() bool {
	return wa.AppID != ""
}

func (wa *WolframAlpha) SelfTest() error {
	if !wa.IsConfigured() {
		return ErrIncompleteConfig
	}
	// Make a test query to verify AppID and response data structure
	resp, err := wa.Query(TestTimeoutSec, "pi")
	if errResult := HTTPErrorToResult(resp, err); errResult != nil {
		return errResult.Error
	}
	// In case that AppID is incorrect, WolframAlpha will still respond with HTTP OK, only the response gives a clue.
	lower := strings.ToLower(string(resp.Body))
	if strings.Contains(lower, "invalid appid") || strings.Contains(lower, "error='true'") || strings.Contains(lower, "success='false'") {
		return errors.New(lower)
	}
	return nil
}

func (wa *WolframAlpha) Initialise() error {
	return nil
}

func (wa *WolframAlpha) Trigger() Trigger {
	return ".w"
}

// Call WolframAlpha API to run a query. Return HTTP status, response, and error if any.
func (wa *WolframAlpha) Query(timeoutSec int, query string) (resp httpclient.Response, err error) {
	resp, err = httpclient.DoHTTP(
		httpclient.Request{TimeoutSec: timeoutSec},
		"https://api.wolframalpha.com/v2/query?appid=%s&input=%s&format=plaintext",
		wa.AppID, query)
	return
}

func (wa *WolframAlpha) Execute(cmd Command) *Result {
	if errResult := cmd.Trim(); errResult != nil {
		return errResult
	}

	resp, err := wa.Query(cmd.TimeoutSec, cmd.Content)
	if errResult := HTTPErrorToResult(resp, err); errResult != nil {
		return errResult
	} else if text, err := wa.ExtractResponse(resp.Body); err != nil {
		return &Result{Error: err, Output: string(resp.Body)}
	} else {
		return &Result{Error: nil, Output: text}
	}
}

// Extract information "pods" from WolframAlpha API response in XML.
func (wa *WolframAlpha) ExtractResponse(xmlBody []byte) (string, error) {
	// Extract plain text information
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
		return "", err
	}
	// Compact information from all pods into a single string
	var outBuf bytes.Buffer
	for _, pod := range result.Pods {
		for _, subPod := range pod.SubPods {
			// Compact pod's key+value ("key | value") by eliminating the pipe symbol
			outBuf.WriteString(strings.TrimSpace(strings.Replace(subPod.TextInfo, " |", "", -1)))
			// Terminate a piece of pod info with full stop
			outBuf.WriteRune('.')
		}
	}
	return outBuf.String(), nil
}
