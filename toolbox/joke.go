package toolbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/inet"
	"strings"
	"time"
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
	result := joke.Execute(Command{})
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
func (joke *Joke) Execute(cmd Command) *Result {
	// Make at most 5 attempts at getting a joke
	for i := 0; i < 5; i++ {
		src := jokeSources[int(time.Now().UnixNano())%len(jokeSources)]
		text, err := src(cmd.TimeoutSec)
		if err == nil {
			return &Result{Output: text}
		}
	}
	return &Result{Error: errors.New("the jokes API are not responding - try again later")}
}

// jokeSources contains functions for retrieving different kinds of jokes
var jokeSources = []func(int) (string, error){getChuckNorrisJoke, getDadJoke, getGenericJoke}

// getChuckNorrisJoke grabs a chuck norris joke from chucknorris.io and returns the joke text.
func getChuckNorrisJoke(timeoutSec int) (string, error) {
	resp, err := inet.DoHTTP(inet.HTTPRequest{TimeoutSec: timeoutSec}, "https://api.chucknorris.io/jokes/random")
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
func getDadJoke(timeoutSec int) (string, error) {
	resp, err := inet.DoHTTP(inet.HTTPRequest{
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

// getGenericJoke grabs a generic joke of any kind from github.com/15Dkatz/official_joke_api, and returns the joke text.
func getGenericJoke(timeoutSec int) (string, error) {
	resp, err := inet.DoHTTP(inet.HTTPRequest{TimeoutSec: timeoutSec}, "https://08ad1pao69.execute-api.us-east-1.amazonaws.com/dev/random_joke")
	if err != nil {
		return "", err
	}
	var respJSON struct {
		Setup     string `json:"setup"`
		Punchline string `json:"punchline"`
	}
	if err = json.Unmarshal(resp.GetBodyUpTo(8192), &respJSON); err != nil {
		return "", err
	}
	text := fmt.Sprintf("%s\n%s", strings.TrimSpace(respJSON.Setup), strings.TrimSpace(respJSON.Punchline))
	if text == "" {
		return "", errors.New("15Dkatz's joke API did not respond with a joke text")
	}
	return text, nil
}
