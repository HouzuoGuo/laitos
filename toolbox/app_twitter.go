package toolbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/HouzuoGuo/laitos/inet"
)

const (
	TwitterGetFeeds  = "g"
	TwitterPostTweet = "p"
)

var (
	RegexTwoNumbers    = regexp.MustCompile(`(\d+)[^\d]+(\d+)`) // Capture two groups of numbers
	ErrBadTwitterParam = fmt.Errorf("example: %s skip# count# | %s content-to-post", TwitterGetFeeds, TwitterPostTweet)
)

// Use Twitter API to interact with user's time-line.
type Twitter struct {
	AccessToken       string `json:"AccessToken"`       // Twitter API access token ("Your Access Token - Access Token")
	AccessTokenSecret string `json:"AccessTokenSecret"` // Twitter API access token secret ("Your Access Token - Access Token Secret")
	ConsumerKey       string `json:"ConsumerKey"`       // Twitter API consumer key ("Application Settings - Consumer Key (API Key)")
	ConsumerSecret    string `json:"ConsumerSecret"`    // Twitter API consumer secret ("Application Settings - Consumer Secret (API Secret)")
	reqSigner         *inet.OAuthSigner
}

var TestTwitter = Twitter{} // API credentials are set by init_feature_test.go

func (twi *Twitter) IsConfigured() bool {
	return twi.AccessToken != "" && twi.AccessTokenSecret != "" &&
		twi.ConsumerKey != "" && twi.ConsumerSecret != ""
}

func (twi *Twitter) SelfTest() error {
	if !twi.IsConfigured() {
		return ErrIncompleteConfig
	}
	// Make an inexpensive API call to test API credentials
	resp, err := inet.DoHTTP(context.Background(), inet.HTTPRequest{
		TimeoutSec: SelfTestTimeoutSec,
		RequestFunc: func(req *http.Request) error {
			return twi.reqSigner.SetAuthorizationHeader(req)
		},
	}, "https://api.twitter.com/1.1/statuses/user_timeline.json?count=1")
	if err != nil {
		return fmt.Errorf("Twitter.SelfTest: API IO error - %v", err)
	}
	if err = resp.Non2xxToError(); err != nil {
		return fmt.Errorf("Twitter.SelfTest: API response error - %v", err)
	}
	return nil
}

func (twi *Twitter) Initialise() error {
	// Initialise API request signer
	twi.reqSigner = &inet.OAuthSigner{
		AccessToken:       twi.AccessToken,
		AccessTokenSecret: twi.AccessTokenSecret,
		ConsumerKey:       twi.ConsumerKey,
		ConsumerSecret:    twi.ConsumerSecret,
	}
	return nil
}

func (twi *Twitter) Trigger() Trigger {
	return ".t"
}

func (twi *Twitter) Execute(ctx context.Context, cmd Command) (ret *Result) {
	if errResult := cmd.Trim(); errResult != nil {
		ret = errResult
		return
	}

	if cmd.FindAndRemovePrefix(TwitterGetFeeds) {
		ret = twi.GetFeeds(ctx, cmd)
	} else if cmd.FindAndRemovePrefix(TwitterPostTweet) {
		ret = twi.Tweet(ctx, cmd)
	} else {
		ret = &Result{Error: ErrBadTwitterParam}
	}
	return
}

// Retrieve tweets from timeline.
func (twi *Twitter) GetFeeds(ctx context.Context, cmd Command) *Result {
	// Find two numeric parameters among the content
	var skip, count int
	params := RegexTwoNumbers.FindStringSubmatch(cmd.Content)
	if len(params) >= 3 {
		var intErr error
		skip, intErr = strconv.Atoi(params[1])
		if intErr != nil {
			return &Result{Error: ErrBadTwitterParam}
		}
		count, intErr = strconv.Atoi(params[2])
		if intErr != nil {
			return &Result{Error: ErrBadTwitterParam}
		}
	}
	// If neither count nor skip was given in the input command, retrieve 10 latest tweets.
	if count == 0 && skip == 0 {
		count = 10
	} else {
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
	}
	// Execute the API request
	resp, err := inet.DoHTTP(ctx, inet.HTTPRequest{
		TimeoutSec: cmd.TimeoutSec,
		RequestFunc: func(req *http.Request) error {
			return twi.reqSigner.SetAuthorizationHeader(req)
		},
	}, "https://api.twitter.com/1.1/statuses/home_timeline.json?count=%s", count)
	// Return error or extract tweets
	if errResult := HTTPErrorToResult(resp, err); errResult != nil {
		return errResult
	} else if tweets, err := twi.ExtractTweets(resp.Body, skip, count); err != nil {
		return &Result{Error: err, Output: string(resp.Body)}
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
func (twi *Twitter) Tweet(ctx context.Context, cmd Command) *Result {
	tweet := cmd.Content
	if tweet == "" {
		return &Result{Error: ErrBadTwitterParam}
	}

	resp, err := inet.DoHTTP(ctx, inet.HTTPRequest{
		TimeoutSec: cmd.TimeoutSec,
		Method:     http.MethodPost,
		RequestFunc: func(req *http.Request) error {
			return twi.reqSigner.SetAuthorizationHeader(req)
		},
	}, "https://api.twitter.com/1.1/statuses/update.json?status=%s", tweet)
	// Return error or extract tweets
	if errResult := HTTPErrorToResult(resp, err); errResult != nil {
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
		finalTweet = len(tweets)
	}
	tweets = tweets[skip:finalTweet]
	return
}
