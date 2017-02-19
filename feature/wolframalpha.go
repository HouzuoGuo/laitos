package feature

import (
	"bytes"
	"encoding/xml"
	"strings"
)

// Send query to WolframAlpha.
type WolframAlpha struct {
	AppID string // Secret application ID granted by WolframAlpha developer console for authorising requests
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
	testExec := wa.Execute(Command{TimeoutSec: 30, Content: "pi"})
	return testExec.Error
}

func (wa *WolframAlpha) Initialise() error {
	return nil
}

func (wa *WolframAlpha) TriggerPrefix() string {
	return ".w"
}

func (wa *WolframAlpha) Execute(cmd Command) (ret *Result) {
	LogBeforeExecute(cmd)
	defer func() {
		LogAfterExecute(cmd, ret)
	}()
	if errResult := cmd.Trim(); errResult != nil {
		ret = errResult
		return
	}

	status, resp, err := DoHTTP(cmd.TimeoutSec, "GET", "application/x-www-form-urlencoded; charset=UTF-8", nil, nil,
		"https://api.wolframalpha.com/v2/query?appid=%s&input=%s&format=plaintext", wa.AppID, cmd.Content)
	if errResult := HTTPResponseError(status, resp, err); errResult != nil {
		ret = errResult
	} else if text, err := wa.ExtractResponse(resp); err != nil {
		ret = &Result{Error: err, Output: string(resp)}
	} else {
		ret = &Result{Error: nil, Output: text}
	}
	return
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
