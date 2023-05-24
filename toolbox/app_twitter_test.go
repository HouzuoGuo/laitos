package toolbox

import (
	"context"
	"reflect"
	"strconv"
	"testing"
)

func TestTwitter_Execute(t *testing.T) {
	if !TestTwitter.IsConfigured() {
		t.Skip("twitter is not configured")
	}
	if err := TestTwitter.Initialise(); err != nil {
		t.Fatal(err)
	}

	if err := TestTwitter.SelfTest(); err != nil {
		t.Fatal(err)
	}

	userID, err := TestTwitter.myUserID(context.Background())
	t.Log(userID, err)

	// Nothing to do
	if ret := TestTwitter.Execute(context.Background(), Command{TimeoutSec: 30, Content: "!@$!@%#%#$@%"}); ret.Error != ErrBadTwitterParam {
		t.Fatal(ret)
	}

	// Retrieve 10 latest tweets
	if ret := TestTwitter.Execute(context.Background(), Command{TimeoutSec: 30, Content: TwitterGetFeeds}); ret.Error != nil ||
		len(ret.Output) < 100 || len(ret.Output) > 3000 {
		t.Fatal(ret)
	}
	// Bad number - still retrieve 10 latest tweets
	if ret := TestTwitter.Execute(context.Background(), Command{TimeoutSec: 30, Content: TwitterGetFeeds + "a, b"}); ret.Error != nil ||
		len(ret.Output) < 100 || len(ret.Output) > 3000 {
		t.Fatal(ret)
	}
	// Retrieve 5 tweets after skipping the latest three tweets
	if ret := TestTwitter.Execute(context.Background(), Command{TimeoutSec: 30, Content: TwitterGetFeeds + "3, 5"}); ret.Error != nil ||
		len(ret.Output) < 100 || len(ret.Output) > 3000 {
		t.Fatal(ret)
	}
	// Posting an empty tweet should result in error
	if ret := TestTwitter.Execute(context.Background(), Command{TimeoutSec: 30, Content: TwitterPostTweet + "  "}); ret.Error != ErrBadTwitterParam {
		t.Fatal(ret)
	}
	// Post a good tweet
	tweet := "laitos twitter test pls ignore"
	if ret := TestTwitter.Execute(context.Background(), Command{TimeoutSec: 30, Content: TwitterPostTweet + tweet}); ret.Error != nil ||
		ret.Output != strconv.Itoa(len(tweet)) {
		t.Fatal(ret)
	}
}

func TestTwitter_ExtractTweets(t *testing.T) {
	input := `{
	"data": [{
		"edit_history_tweet_ids": ["1661425205128855576"],
		"id": "1661425205128855576",
		"text": "Amazon creeps into the premium tablet market with the Fire Max 11 https://t.co/6CybSiDggy by @RonAmadeo",
		"author_id": "717313"
	}, {
		"edit_history_tweet_ids": ["1661423839665098754"],
		"id": "1661423839665098754",
		"text": "Settlement of €1.25m for man who sued over hospital care after roof fall https://t.co/UtftVyQqR1",
		"author_id": "15084853"
	}],
	"includes": {
		"users": [{
			"id": "15084853",
			"name": "The Irish Times",
			"username": "IrishTimes"
		}, {
			"id": "717313",
			"name": "Ars Technica",
			"username": "arstechnica"
		}]
	},
	"meta": {
		"next_token": "7140dibdnow9c7btw452ufatgqvyqqyrqk7rlo41w7ak4",
		"result_count": 10,
		"newest_id": "1661428577391329283",
		"oldest_id": "1661423839665098754"
	}
}`
	got2, err := TestTwitter.ExtractTweets([]byte(input), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	want2 := []Tweet{
		{
			Text:     "Amazon creeps into the premium tablet market with the Fire Max 11 https://t.co/6CybSiDggy by @RonAmadeo",
			UserName: "arstechnica",
		},
		{
			Text:     "Settlement of €1.25m for man who sued over hospital care after roof fall https://t.co/UtftVyQqR1",
			UserName: "IrishTimes",
		},
	}
	if !reflect.DeepEqual(got2, want2) {
		t.Fatalf("\nGot:\n%+v\nWant:\n%+v\n", got2, want2)
	}

	got1, err := TestTwitter.ExtractTweets([]byte(input), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	want1 := []Tweet{
		{
			Text:     "Settlement of €1.25m for man who sued over hospital care after roof fall https://t.co/UtftVyQqR1",
			UserName: "IrishTimes",
		},
	}
	if !reflect.DeepEqual(got1, want1) {
		t.Fatalf("Got:\n%+v\nWant:\n%+v", got1, want1)
	}

	got0, err := TestTwitter.ExtractTweets([]byte(input), 2, 100)
	if err != nil || len(got0) != 0 {
		t.Fatalf("Got:\n%+v", got0)
	}
}
