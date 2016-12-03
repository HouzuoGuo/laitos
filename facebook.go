package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Use Facebook API to post a status update.
type FacebookClient struct {
	AccessToken string
}

// Post a Facebook status update.
func (fb *FacebookClient) WriteStatus(apiTimeoutSec int, updateMessage string) error {
	client := &http.Client{Timeout: time.Duration(apiTimeoutSec) * time.Second}
	request, err := http.NewRequest(
		"POST",
		"https://graph.facebook.com//v2.8/me/feed?access_token="+fb.AccessToken,
		strings.NewReader(url.Values{"message": []string{updateMessage}}.Encode()))
	if err != nil {
		return err
	}
	// Execute and read JSON response
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	log.Printf("Status update request got a response: error %v, status %d, output %d", err, response.StatusCode, body)
	if response.StatusCode/200 != 1 {
		return fmt.Errorf("Status update API responded with status code %d", response.StatusCode)
	}
	return nil
}
