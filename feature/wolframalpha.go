package feature

import (
	"bytes"
	"encoding/xml"
	"errors"
	"log"
	"strings"
)

// Implement Result interface for WolframAlpha query.
type WolframAlphaResult struct {
	Error     error
	RawOutput []byte
}

func (waResult *WolframAlphaResult) Err() error {
	return waResult.Error
}

func (waResult *WolframAlphaResult) ErrText() string {
	if waResult.Error == nil {
		return ""
	}
	return waResult.Error.Error()
}

func (waResult *WolframAlphaResult) OutText() string {
	if waResult.RawOutput == nil {
		return ""
	}
	return string(waResult.RawOutput)
}

func (waResult *WolframAlphaResult) CombinedText() (ret string) {
	errText := waResult.ErrText()
	outText := waResult.OutText()
	if errText != "" {
		ret = errText
		if outText != "" {
			ret += COMBINED_TEXT_SEP
		}
	}
	ret += outText
	return
}

// Query WolframAlpha.
type WolframAlpha struct {
	AppID string // Secret application ID granted by WolframAlpha developer console for authorising requests
}

func (wa *WolframAlpha) InitAndTest() error {
	if wa.AppID == "" {
		return errors.New("WolframAlpha AppID is empty")
	}
	// TODO: call WolframAlpha to validate AppID
	return nil
}

func (wa *WolframAlpha) TriggerPrefix() string {
	return ".w"
}

func (wa *WolframAlpha) Execute(timeoutSec int, query string) (ret Result) {
	log.Printf("WolframAlpha.Execute: will run query - %s", query)
	return nil
}

// Extract "pods" from WolframAlpha API response in XML.
func (wa *WolframAlpha) ExtractResponse(xmlBody []byte) string {
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
			// Further compact output by eliminating " |" from pods
			outBuf.WriteString(strings.TrimSpace(strings.Replace(subPod.TextInfo, " |", "", -1)))
			outBuf.WriteRune('.')
		}
	}
	return outBuf.String()
}
