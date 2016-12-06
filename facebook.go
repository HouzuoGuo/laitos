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
	urlPath := "https://graph.facebook.com/v2.8/me/feed?access_token=" + fb.AccessToken
	request, err := http.NewRequest("POST", urlPath, strings.NewReader(url.Values{"message": []string{updateMessage}}.Encode()))
	if err != nil {
		return err
	}
	// Execute and read JSON response
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	log.Printf("Facebook responded to '%s': error %v, status %d, output %s", updateMessage, err, response.StatusCode, string(body))
	if response.StatusCode/200 != 1 {
		return fmt.Errorf("HTTP status code %d", response.StatusCode)
	}
	return nil
}
