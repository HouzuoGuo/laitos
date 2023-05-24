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
	TwitterGetFeeds       = "g"
	TwitterPostTweet      = "p"
	TwitterMaxResponseLen = 256 * 1024
)

var (
	RegexTwoNumbers    = regexp.MustCompile(`(\d+)[^\d]+(\d+)`) // Capture two groups of numbers
	ErrBadTwitterParam = fmt.Errorf("example: %s skip# count# | %s content-to-post", TwitterGetFeeds, TwitterPostTweet)
)

// Use Twitter API to interact with user's time-line.
type Twitter struct {
	// AccessToken is my own twitter user's access token.
	AccessToken string `json:"AccessToken"`
	// AccessToken is my own twitter user's access token secret.
	AccessTokenSecret string `json:"AccessTokenSecret"`
	// ConsumerKey is the the twitter app's consumer API key.
	ConsumerKey string `json:"ConsumerKey"`
	// ConsumerKey is the the twitter app's consumer API secret.
	ConsumerSecret string `json:"ConsumerSecret"`
	// MyUserName is my own twitter user name for retrieving my home timeline.
	MyUserName string `json:"MyUserName"`
	reqSigner  *inet.OAuthSigner
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
	// User ID look-up is an inexpensive API call, sufficient for validating the API credentials.
	_, err := twi.myUserID(context.Background())
	if err != nil {
		return fmt.Errorf("Twitter.SelfTest: API IO error - %v", err)
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

func (twi *Twitter) myUserID(ctx context.Context) (string, error) {
	resp, err := inet.DoHTTP(ctx, inet.HTTPRequest{
		MaxBytes: TwitterMaxResponseLen,
		RequestFunc: func(req *http.Request) error {
			return twi.reqSigner.SetAuthorizationHeader(req)
		},
	}, "https://api.twitter.com/2/users/by/username/%s", twi.MyUserName)
	if err != nil {
		return "", err
	} else if err := resp.Non2xxToError(); err != nil {
		return "", err
	}
	var respObj struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.GetBodyUpTo(1024*128), &respObj); err != nil {
		return "", err
	}
	return respObj.Data.ID, nil
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
		// The API will not retrieve more than 100 tweets at a time.
		if skip > 99 {
			skip = 99
		}
		if skip < 0 {
			skip = 0
		}
		count += skip
		if count > 100 {
			count = 100
		}
		if count < 1 {
			count = 1
		}
	}
	userID, err := twi.myUserID(ctx)
	if err != nil {
		return &Result{Error: err}
	}
	// Execute the API request
	resp, err := inet.DoHTTP(ctx, inet.HTTPRequest{
		TimeoutSec: cmd.TimeoutSec,
		MaxBytes:   TwitterMaxResponseLen,
		RequestFunc: func(req *http.Request) error {
			return twi.reqSigner.SetAuthorizationHeader(req)
		},
	}, "https://api.twitter.com/2/users/%s/timelines/reverse_chronological?max_results=%s&user.fields=name&expansions=author_id", userID, strconv.Itoa(count))
	// Return error or extract tweets
	if errResult := HTTPErrorToResult(resp, err); errResult != nil {
		return errResult
	} else if tweets, err := twi.ExtractTweets(resp.Body, skip, count); err != nil {
		return &Result{Error: err, Output: string(resp.Body)}
	} else {
		// Return one tweet per line
		var outBuf bytes.Buffer
		for _, tweet := range tweets {
			outBuf.WriteString(fmt.Sprintf("%s %s\n", strings.TrimSpace(tweet.UserName), strings.TrimSpace(tweet.Text)))
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
	reqBody, err := json.Marshal(map[string]string{"text": tweet})
	if err != nil {
		return &Result{Error: err}
	}
	resp, err := inet.DoHTTP(ctx, inet.HTTPRequest{
		TimeoutSec:  cmd.TimeoutSec,
		Method:      http.MethodPost,
		MaxBytes:    TwitterMaxResponseLen,
		Body:        bytes.NewReader(reqBody),
		ContentType: "application/json",
		RequestFunc: func(req *http.Request) error {
			return twi.reqSigner.SetAuthorizationHeader(req)
		},
	}, "https://api.twitter.com/2/tweets")
	// Return error or extract tweets
	if errResult := HTTPErrorToResult(resp, err); errResult != nil {
		return errResult
	}
	// The OK output is simply the length of trimmed tweet
	return &Result{Output: strconv.Itoa(len(tweet))}
}

type Tweet struct {
	Text     string
	UserName string
}

func (twi *Twitter) ExtractTweets(jsonBody []byte, skip, count int) (tweets []Tweet, err error) {
	var apiResp struct {
		Data []struct {
			Text     string `json:"text"`
			AuthorID string `json:"author_id"`
		} `json:"data"`
		Includes struct {
			Users []struct {
				ID       string `json:"id"`
				UserName string `json:"username"`
			} `json:"users"`
		} `json:"includes"`
	}
	if err = json.Unmarshal(jsonBody, &apiResp); err != nil {
		return
	}
	// Construct the author ID to user name mapping.
	authorIDUserName := make(map[string]string)
	for _, user := range apiResp.Includes.Users {
		authorIDUserName[user.ID] = user.UserName
	}
	// Turn response data entries into tweets.
	for _, entry := range apiResp.Data {
		tweets = append(tweets, Tweet{
			Text:     entry.Text,
			UserName: authorIDUserName[entry.AuthorID],
		})
	}
	// Skipping all tweets?
	if skip >= len(tweets) {
		tweets = []Tweet{}
		return
	}
	finalTweet := skip + count
	if finalTweet > len(tweets) {
		finalTweet = len(tweets)
	}
	tweets = tweets[skip:finalTweet]
	return
}
