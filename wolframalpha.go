package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Invoke WolframAlpha for processing questions.
type WolframAlphaClient struct {
	AppID string // Secret application ID granted by WolframAlpha developer console for authorising requests
}

// Extract "pods" from WolframAlpha API response in XML.
func (wa *WolframAlphaClient) ExtractResponse(xmlBody []byte) string {
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

// Call WolframAlpha API with the text query.
func (wa *WolframAlphaClient) InvokeAPI(timeoutSec int, query string) (out string, err error) {
	request, err := http.NewRequest(
		"GET",
		fmt.Sprintf("https://api.wolframalpha.com/v2/query?appid=%s&input=%s&format=plaintext", wa.AppID, url.QueryEscape(query)),
		bytes.NewReader([]byte{}))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}
	defer response.Body.Close()

	log.Printf("WolframAlpha responded to '%s': error %v, status %d, output length %d", query, err, response.StatusCode, len(body))
	out = wa.ExtractResponse(body)
	if response.StatusCode/200 != 1 {
		err = fmt.Errorf("HTTP status code %d", response.StatusCode)
	}
	return
}
