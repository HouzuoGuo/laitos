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

// Invoke Twilio API for sending texts and making calls.
type TwilioClient struct {
	PhoneNumber string // Twilio telephone number for outbound call and SMS
	AccountSID  string // Twilio account SID
	AuthSecret  string // Twilio authentication secret token
}

func (twilio *TwilioClient) InvokeAPI(timeoutSec int, finalEndpoint string, toNumber string, otherParams map[string]string) error {
	urlParams := url.Values{"From": []string{twilio.PhoneNumber}, "To": []string{toNumber}}
	for key, val := range otherParams {
		urlParams[key] = []string{val}
	}
	urlPath := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/%s", twilio.AccountSID, finalEndpoint)
	request, err := http.NewRequest("POST", urlPath, strings.NewReader(urlParams.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	request.SetBasicAuth(twilio.AccountSID, twilio.AuthSecret)
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	log.Printf("Twilio responded to %s (%s): error %v, status %d, output %s", finalEndpoint, toNumber, err, response.StatusCode, string(body))
	if response.StatusCode/200 != 1 {
		return fmt.Errorf("HTTP status code %d", response.StatusCode)
	}
	return nil
}

// Send an SMS via Twilio.
func (twilio *TwilioClient) SendText(apiTimeoutSec int, toNumber string, message string) error {
	return twilio.InvokeAPI(apiTimeoutSec, "Messages.json", toNumber, map[string]string{
		"Body": message})
}

func (twilio *TwilioClient) MakeCall(apiTimeoutSec int, toNumber string, message string) error {
	return twilio.InvokeAPI(apiTimeoutSec, "Calls.json", toNumber, map[string]string{
		"Url": "http://twimlets.com/message?Message=" + url.QueryEscape(message+" repeat once more "+message)})
}
