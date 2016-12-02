package main

import (
	"encoding/json"
	"fmt"
	"github.com/HouzuoGuo/websh/oauth"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type TwitterClient struct {
	APIConsumerKey       string
	APIConsumerSecret    string
	APIAccessToken       string
	APIAccessTokenSecret string
}

type Tweet struct {
	Text string `json:"text"`
	User struct {
		Name string `json:"name"`
	} `json:"user"`
}

// Retrieve the latest tweets. Due to Twitter limitation it will not look back further than 200 tweets.
func (twi *TwitterClient) RetrieveLatest(apiTimeoutSec, skip, count int) (tweets []Tweet, err error) {
	if skip > 199 {
		skip = 199
	}
	if count > 200 {
		count = 200
	}
	// Create and sign API request
	auther := &oauth.AuthHead{
		ConsumerKey:       twi.APIConsumerKey,
		ConsumerSecret:    twi.APIConsumerSecret,
		AccessToken:       twi.APIAccessToken,
		AccessTokenSecret: twi.APIAccessTokenSecret,
	}
	client := &http.Client{Timeout: time.Duration(apiTimeoutSec) * time.Second}
	request, err := http.NewRequest("GET", "https://api.twitter.com/1.1/statuses/home_timeline.json?count="+strconv.Itoa(count), nil)
	if err != nil {
		return
	} else if err = auther.SetRequestAuthHeader(request); err != nil {
		return
	}
	// Execute and read JSON response
	response, err := client.Do(request)
	if err != nil {
		return
	}
	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	log.Printf("Timeline retrieval request got a response: error %v, status %d, output length %d", err, response.StatusCode, len(body))
	if response.StatusCode/200 != 1 {
		err = fmt.Errorf("Timeline retrieval API responded with status code %d", response.StatusCode)
		return
	}
	// Skip certain number of tweets
	if err = json.Unmarshal(body, &tweets); err != nil {
		return
	}

	// I have no idea why Twitter cannot count. The API won't return the exact number of tweets as requested.
	numKeep := count - skip
	if len(tweets) < numKeep {
		numKeep = len(tweets)
	}
	if numKeep < 1 {
		numKeep = 1
	}
	tweets = tweets[:numKeep]

	return
}

// Post a tweet.
func (twi *TwitterClient) PostUpdate(apiTimeoutSec int, text string) error {
	// Create and sign API request
	auther := &oauth.AuthHead{
		ConsumerKey:       twi.APIConsumerKey,
		ConsumerSecret:    twi.APIConsumerSecret,
		AccessToken:       twi.APIAccessToken,
		AccessTokenSecret: twi.APIAccessTokenSecret,
	}
	client := &http.Client{Timeout: time.Duration(apiTimeoutSec) * time.Second}
	request, err := http.NewRequest("POST", "https://api.twitter.com/1.1/statuses/update.json?status="+url.QueryEscape(text), nil)
	if err != nil {
		return err
	} else if err = auther.SetRequestAuthHeader(request); err != nil {
		return err
	}
	// Execute and read JSON response
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	log.Printf("Timeline update request got a response: error %v, status %d, output length %d", err, response.StatusCode, len(body))
	if response.StatusCode/200 != 1 {
		return fmt.Errorf("Timeline update API responded with status code %d", response.StatusCode)
	}
	return nil
}
