package toolbox

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	"github.com/HouzuoGuo/laitos/inet"
)

// Send query to WolframAlpha.
type WolframAlpha struct {
	AppID string `json:"AppID"` // WolframAlpha API AppID ("Developer Portal - My Apps - <name> - AppID")
}

var TestWolframAlpha = WolframAlpha{} // AppID is set by init_feature_test.go

func (wa *WolframAlpha) IsConfigured() bool {
	return wa.AppID != ""
}

func (wa *WolframAlpha) SelfTest() error {
	if !wa.IsConfigured() {
		return ErrIncompleteConfig
	}
	// Make a test query to verify AppID and response data structure
	resp, err := wa.Query(SelfTestTimeoutSec, "pi")
	if errResult := HTTPErrorToResult(resp, err); errResult != nil {
		return fmt.Errorf("WolframAlpha.SelfTest: query result error - %v", errResult.Error)
	}
	// In case that AppID is incorrect, WolframAlpha will still respond with HTTP OK, only the response gives a clue.
	lower := strings.ToLower(string(resp.Body))
	if strings.Contains(lower, "invalid appid") || strings.Contains(lower, "error='true'") || strings.Contains(lower, "success='false'") {
		return errors.New("WolframAlpha.SelfTest: AppID appears to be incorrect")
	}
	return nil
}

func (wa *WolframAlpha) Initialise() error {
	return nil
}

func (wa *WolframAlpha) Trigger() Trigger {
	return ".w"
}

// AtLeast returns i only if it is larger than atLeast. If not, it returns atLeast.
func AtLeast(i, atLeast int) int {
	if i < atLeast {
		return atLeast
	}
	return i
}

// Call WolframAlpha API to run a query. Return HTTP status, response, and error if any.
func (wa *WolframAlpha) Query(timeoutSec int, query string) (resp inet.HTTPResponse, err error) {
	// The following ratios are inspired by default timeout settings from WolframAlpha API
	scanTimeout := AtLeast(timeoutSec*3/20, 2)
	podTimeout := AtLeast(timeoutSec*4/20, 2)
	formatTimeout := AtLeast(timeoutSec*8/20, 4)
	parseTimeout := AtLeast(timeoutSec*5/20, 3)
	// Leave 2 seconds of buffer time for transmitting the feature response back to user
	totalTimeout := AtLeast(timeoutSec-2, 2)
	resp, err = inet.DoHTTP(
		inet.HTTPRequest{TimeoutSec: timeoutSec},
		"https://api.wolframalpha.com/v2/query?appid=%s&input=%s&format=plaintext&scantimeout=%s&podtimeout=%s&formattimeout=%s&parsetimeout=%s&totaltimeout=%s&reinterpret=true&translation=true&ignorecase=true",
		wa.AppID, query, scanTimeout, podTimeout, formatTimeout, parseTimeout, totalTimeout)
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
		// Print titles in upper case to improve their visibility
		if pod.Title != "" {
			outBuf.WriteString(fmt.Sprintf("(%s) ", strings.ToUpper(pod.Title)))
		}
		for _, subPod := range pod.SubPods {
			/*
				In an individual subpod, the pipe symbol is often used as a separator between item and description.
				However for text response there are often entirely empty items without an item name and description,
				leaving only the pipe symbol in place.
				Therefore, remove all instances of the pipe symbol to make the text response much more readable.
			*/
			// In an individual Turn pod response "Item | Item Description" into "Item: Item Description" for better readability
			text := strings.TrimSpace(strings.Replace(subPod.TextInfo, " |", "", -1))
			if text == "" {
				continue
			}
			if subPod.Title != "" {
				outBuf.WriteString(fmt.Sprintf("[%s] ", strings.ToUpper(pod.Title)))
			}
			outBuf.WriteString(text)
			outBuf.WriteRune('\n')
		}
		outBuf.WriteRune('\n')
	}
	return outBuf.String(), nil
}
