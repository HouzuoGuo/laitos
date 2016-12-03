package main

import (
	"fmt"
	"testing"
)

func TestTwitter(t *testing.T) {
	t.Skip()
	twitter := &TwitterClient{
		APIConsumerKey:       "FILLME",
		APIConsumerSecret:    "FILLME",
		APIAccessToken:       "FILLME",
		APIAccessTokenSecret: "FILLME",
	}
	// Retrieve timeline
	tweets, err := twitter.RetrieveLatest(10, 10, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(tweets) != 10 {
		fmt.Println("not enough tweets")
	}
	for _, t := range tweets {
		fmt.Println(t)
	}
	// Post update
	err = twitter.Tweet(10, "aaaaa")
	if err != nil {
		t.Fatal(err)
	}
}
