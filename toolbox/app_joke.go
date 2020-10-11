package toolbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
)

// Joke as a toolbox feature takes no input and responds with one joke text downloaded from a list of known sources.
type Joke struct {
}

// IsConfigured always returns true because configuration is not required for this feature.
func (joke *Joke) IsConfigured() bool {
	return true
}

// SelfTest tries to grab a joke text, and returns an error only if it fails in doing so.
func (joke *Joke) SelfTest() error {
	result := joke.Execute(context.Background(), Command{})
	return result.Error
}

// Initialise does nothing because initialisation is not required for this feature.
func (joke *Joke) Initialise() error {
	return nil
}

// Trigger returns the trigger prefix string ".j", consistent with the feature name.
func (joke *Joke) Trigger() Trigger {
	return ".j"
}

/*
Execute takes timeout from input command and uses it when retrieving joke text. It retrieves exactly one joke text from
a randomly chosen source. It retries up to 5 times in case a source does not respond.
*/
func (joke *Joke) Execute(ctx context.Context, cmd Command) *Result {
	// Make at most 5 attempts at getting a joke
	for i := 0; i < 5; i++ {
		src := jokeSources[int(time.Now().UnixNano())%len(jokeSources)]
		text, err := src(ctx, cmd.TimeoutSec)
		if err == nil {
			return &Result{Output: text}
		}
	}
	return &Result{Error: errors.New("the jokes API are not responding - try again later")}
}

// jokeSources contains functions for retrieving different kinds of jokes
var jokeSources = []func(context.Context, int) (string, error){getChuckNorrisJoke, getDadJoke}

// getChuckNorrisJoke grabs a chuck norris joke from chucknorris.io and returns the joke text.
func getChuckNorrisJoke(ctx context.Context, timeoutSec int) (string, error) {
	resp, err := inet.DoHTTP(ctx, inet.HTTPRequest{TimeoutSec: timeoutSec}, "https://api.chucknorris.io/jokes/random")
	if err != nil {
		return "", err
	}
	var respJSON struct {
		Value string `json:"value"`
	}
	if err = json.Unmarshal(resp.GetBodyUpTo(8192), &respJSON); err != nil {
		return "", err
	}
	text := strings.TrimSpace(respJSON.Value)
	if text == "" {
		return "", errors.New("chucknorris.io did not respond with a joke text")
	}
	return text, nil
}

// getDadJoke grabs a dad joke from icanhazdadjoke.com and returns the joke text.
func getDadJoke(ctx context.Context, timeoutSec int) (string, error) {
	resp, err := inet.DoHTTP(ctx, inet.HTTPRequest{
		TimeoutSec: timeoutSec,
		Header: map[string][]string{
			"User-Agent": {"laitos (https://github.com/HouzuoGuo/laitos)"},
			"Accept":     {"text/plain"},
		}}, "https://icanhazdadjoke.com/")
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(string(resp.GetBodyUpTo(4096)))
	if text == "" {
		return "", errors.New("icanhazdadjoke.com did not respond with a joke text")
	}
	return text, nil
}
