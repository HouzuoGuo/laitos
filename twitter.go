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

// Invoke Twitter API to interact with user's time-line.
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

// Retrieve the latest tweets.
func (twi *TwitterClient) RetrieveLatest(apiTimeoutSec, skip, count int) (tweets []Tweet, err error) {
	// Due to Twitter limitation, it will not look back further than 200 tweets.
	if skip > 199 {
		skip = 199
	}
	if skip < 0 {
		skip = 0
	}
	count += skip
	if count > 200 {
		count = 200
	}
	if count < 1 {
		count = 1
	}
	// Create and sign API request
	auther := &oauth.AuthHead{
		ConsumerKey:       twi.APIConsumerKey,
		ConsumerSecret:    twi.APIConsumerSecret,
		AccessToken:       twi.APIAccessToken,
		AccessTokenSecret: twi.APIAccessTokenSecret,
	}
	client := &http.Client{Timeout: time.Duration(apiTimeoutSec) * time.Second}
	urlPath := "https://api.twitter.com/1.1/statuses/home_timeline.json?count=" + strconv.Itoa(count)
	request, err := http.NewRequest("GET", urlPath, nil)
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
	if err != nil {
		return
	}
	defer response.Body.Close()

	log.Printf("Twitter timeline responded to (%d,%d): error %v, status %d, output length %d", skip, count, err, response.StatusCode, len(body))
	if response.StatusCode/200 != 1 {
		err = fmt.Errorf("HTTP status code %d", response.StatusCode)
		return
	}

	// Skip certain number of tweets
	if err = json.Unmarshal(body, &tweets); err != nil {
		return
	}
	if skip >= len(tweets) {
		tweets = []Tweet{}
		return
	}
	finalTweet := count
	if finalTweet > len(tweets) {
		finalTweet = count
	}
	tweets = tweets[skip:finalTweet]
	return
}

// Post a tweet.
func (twi *TwitterClient) Tweet(apiTimeoutSec int, text string) error {
	// Create and sign API request
	auther := &oauth.AuthHead{
		ConsumerKey:       twi.APIConsumerKey,
		ConsumerSecret:    twi.APIConsumerSecret,
		AccessToken:       twi.APIAccessToken,
		AccessTokenSecret: twi.APIAccessTokenSecret,
	}
	client := &http.Client{Timeout: time.Duration(apiTimeoutSec) * time.Second}
	urlPath := "https://api.twitter.com/1.1/statuses/update.json?status=" + url.QueryEscape(text)
	request, err := http.NewRequest("POST", urlPath, nil)
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
	if err != nil {
		return err
	}
	defer response.Body.Close()

	log.Printf("Twitter update responded to '%s': error %v, status %d, output %s", text, err, response.StatusCode, string(body))
	if response.StatusCode/200 != 1 {
		return fmt.Errorf("HTTP status code %d", response.StatusCode)
	}
	return nil
}
