package feature

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/websh/oauth"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const (
	TWITTER_GET_FEEDS = "g"
	TWITTER_TWEET     = "p"
)

var RegexTwoNumbers = regexp.MustCompile(`([0-9]+)[^0-9]+([0-9]+)`) // Capture two groups of numbers

// Use Twitter API to interact with user's time-line.
type Twitter struct {
	APIAccessToken       string
	APIAccessTokenSecret string
	APIConsumerKey       string
	APIConsumerSecret    string
	reqSigner            *oauth.AuthHead
}

var TestTwitter = Twitter{} // API credentials are set by init_test.go

func (twi *Twitter) IsConfigured() bool {
	return twi.APIAccessToken != "" && twi.APIAccessTokenSecret != "" &&
		twi.APIConsumerKey != "" && twi.APIConsumerSecret != ""
}

func (twi *Twitter) Initialise() error {
	log.Print("Twitter.Initialise: in progress")
	if !twi.IsConfigured() {
		return ErrIncompleteConfig
	}
	// Initialise API request signer
	twi.reqSigner = &oauth.AuthHead{
		AccessToken:       twi.APIAccessToken,
		AccessTokenSecret: twi.APIAccessTokenSecret,
		ConsumerKey:       twi.APIConsumerKey,
		ConsumerSecret:    twi.APIConsumerSecret,
	}
	// Make a test query (retrieve one tweet) to verify validity of API credentials
	testExec := twi.Execute(Command{TimeoutSec: 30, Content: TWITTER_GET_FEEDS})
	if testExec.Error != nil {
		return testExec.Error
	}
	log.Printf("Twitter.Initialise: successfully completed (test query returned %d characters)", len(testExec.Output))
	return nil
}

func (twi *Twitter) TriggerPrefix() string {
	return ".t"
}

func (twi *Twitter) Execute(cmd Command) (ret *Result) {
	LogBeforeExecute(cmd)
	defer func() {
		LogAfterExecute(cmd, ret)
	}()
	if errResult := cmd.Trim(); errResult != nil {
		ret = errResult
		return
	}

	if strings.HasPrefix(cmd.Content, TWITTER_GET_FEEDS) {
		ret = twi.GetFeeds(cmd)
	} else if strings.HasPrefix(cmd.Content, TWITTER_TWEET) {
		ret = twi.Tweet(cmd)
	} else {
		ret = &Result{Error: fmt.Errorf("Failed to find command prefix (either %s or %s)", TWITTER_GET_FEEDS, TWITTER_TWEET)}
	}
	return
}

// Retrieve tweets from timeline.
func (twi *Twitter) GetFeeds(cmd Command) *Result {
	// Find two numeric parameters among the content
	var skip, count int
	params := RegexTwoNumbers.FindStringSubmatch(cmd.Content)
	if len(params) < 3 {
		// If skip/count are not given or incomplete, retrieve exactly one latest tweet.
		skip = 0
		count = 1
	} else {
		var intErr error
		skip, intErr = strconv.Atoi(params[1])
		if intErr != nil {
			return &Result{Error: fmt.Errorf("First parameter '%s' is not a number", params[1])}
		}
		count, intErr = strconv.Atoi(params[2])
		if intErr != nil {
			return &Result{Error: fmt.Errorf("Second parameter '%s' is not a number", params[2])}
		}
	}
	// Twitter API will not retrieve more than 200 tweets, so limit the parameters accordingly.
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
	// Execute the API request
	status, resp, err := DoHTTP(cmd.TimeoutSec, "GET", "", nil, func(req *http.Request) error {
		return twi.reqSigner.SetRequestAuthHeader(req)
	}, "https://api.twitter.com/1.1/statuses/home_timeline.json?count=%s", count)
	// Return error or extract tweets
	if errResult := HTTPResponseError(status, resp, err); errResult != nil {
		return errResult
	} else if tweets, err := twi.ExtractTweets(resp, skip, count); err != nil {
		return &Result{Error: err, Output: string(resp)}
	} else {
		// Return one tweet per line
		var outBuf bytes.Buffer
		for _, tweet := range tweets {
			outBuf.WriteString(fmt.Sprintf("%s %s\n", strings.TrimSpace(tweet.User.Name), strings.TrimSpace(tweet.Text)))
		}
		return &Result{Error: nil, Output: outBuf.String()}
	}
}

// Post a new tweet to timeline.
func (twi *Twitter) Tweet(cmd Command) *Result {
	tweet := strings.TrimSpace(strings.TrimPrefix(cmd.Content, TWITTER_TWEET))
	if tweet == "" {
		return &Result{Error: errors.New("Post content is empty")}
	}
	status, resp, err := DoHTTP(cmd.TimeoutSec, "POST", "", nil, func(req *http.Request) error {
		return twi.reqSigner.SetRequestAuthHeader(req)
	}, "https://api.twitter.com/1.1/statuses/update.json?status=%s", tweet)
	// Return error or extract tweets
	if errResult := HTTPResponseError(status, resp, err); errResult != nil {
		return errResult
	}
	// The OK output is simply the length of trimmed tweet
	return &Result{Output: strconv.Itoa(len(tweet))}
}

type Tweet struct {
	Text string `json:"text"`
	User struct {
		Name string `json:"name"`
	} `json:"user"`
}

func (twi *Twitter) ExtractTweets(jsonBody []byte, skip, count int) (tweets []Tweet, err error) {
	if err = json.Unmarshal(jsonBody, &tweets); err != nil {
		return
	}
	// Skipping all tweets?
	if skip >= len(tweets) {
		tweets = []Tweet{}
		return
	}
	finalTweet := count
	// Retrieving more tweets than there are in response?
	if finalTweet > len(tweets) {
		finalTweet = count
	}
	tweets = tweets[skip:finalTweet]
	return
}
