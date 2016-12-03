package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

// Intentionally undocumented hehehe.
type MysteriousClient struct {
	URL             string
	Addr1           string
	Addr2           string
	ID1             string
	ID2             string
	Cmds            []string
	CmdIntervalHour int
}

// Return true only if mysterious command process should be enabled.
func (myst *MysteriousClient) IsEnabled() bool {
	return myst.URL != "" && myst.Addr1 != "" && myst.Addr2 != "" && myst.ID1 != "" && myst.ID2 != ""
}

// This mysterious HTTP call is intentionally undocumented hahahaha.
func (myst *MysteriousClient) InvokeAPI(rawMessage string) (err error) {
	requestBody := fmt.Sprintf("ReplyAddress=%s&ReplyMessage=%s&MessageId=%s&Guid=%s",
		url.QueryEscape(myst.Addr2), url.QueryEscape(rawMessage), myst.ID1, myst.ID2)

	request, err := http.NewRequest("POST", myst.URL, bytes.NewReader([]byte(requestBody)))
	if err != nil {
		return
	}
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/52.0.2743.116 Safari/537.36")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	client := &http.Client{Timeout: 25 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return
	}
	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()

	log.Printf("Mysterious thingy responded to '%s' : error %v, status %d, output %s", requestBody, err, response.StatusCode, string(body))
	return
}
