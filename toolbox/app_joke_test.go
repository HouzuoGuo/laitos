package toolbox

import (
	"context"
	"testing"
)

func TestJokeSources(t *testing.T) {
	text, err := getDadJoke(context.Background(), 10)
	if err != nil || text == "" {
		t.Fatal(err)
	}
	text, err = getChuckNorrisJoke(context.Background(), 10)
	if err != nil || text == "" {
		t.Fatal(err)
	}
}

func TestJoke(t *testing.T) {
	joke := Joke{}
	if !joke.IsConfigured() {
		t.Fatal("should be configured")
	}
	if err := joke.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := joke.SelfTest(); err != nil {
		t.Fatal(err)
	}

	if result := joke.Execute(context.Background(), Command{TimeoutSec: 10}); result.Error != nil || len(result.Output) < 10 {
		t.Fatal(result)
	} else {
		t.Log(result)
	}
}
