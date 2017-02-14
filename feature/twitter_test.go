package feature

import (
	"strconv"
	"strings"
	"testing"
)

func TestTwitter_Execute(t *testing.T) {
	if !TestTwitter.IsConfigured() {
		t.Skip()
	}
	if err := TestTwitter.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Nothing to do
	if ret := TestTwitter.Execute(Command{TimeoutSec: 30, Content: "!@$!@%#%#$@%"}); ret.Error == nil {
		t.Fatal("did not error")
	}
	// Retrieve one latest tweet
	if ret := TestTwitter.Execute(Command{TimeoutSec: 30, Content: TWITTER_GET_FEEDS}); ret.Error != nil ||
		len(strings.Split(ret.Output, "\n")) != 1+1 {
		t.Fatal(ret)
	}
	// Retrieve 4 tweets after skip the very latest one
	if ret := TestTwitter.Execute(Command{TimeoutSec: 30, Content: TWITTER_GET_FEEDS + "1, 5"}); ret.Error != nil ||
		len(strings.Split(ret.Output, "\n")) != 5+1 {
		t.Fatal(ret)
	}
	// Posting an empty tweet should result in error
	if ret := TestTwitter.Execute(Command{TimeoutSec: 30, Content: TWITTER_TWEET + "  "}); ret.Error == nil ||
		ret.ErrText() != "Post content is empty" {
		t.Fatal(ret)
	}
	// Post a good tweet
	tweet := "test pls ignore"
	if ret := TestTwitter.Execute(Command{TimeoutSec: 30, Content: TWITTER_TWEET + tweet}); ret.Error != nil ||
		ret.Output != strconv.Itoa(len(tweet)) {
		t.Fatal(ret)
	}
}

func TestTwitter_ExtractTweets(t *testing.T) {
	input := `[{
	"created_at": "Sat Feb 11 14:36:57 +0000 2017",
	"id": 830425394977320962,
	"id_str": "830425394977320962",
	"text": "Cambodia opposition leader Sam Rainsy resigns https:\/\/t.co\/FgXF8S97Qt",
	"truncated": false,
	"entities": {
		"hashtags": [],
		"symbols": [],
		"user_mentions": [],
		"urls": [{
			"url": "https:\/\/t.co\/FgXF8S97Qt",
			"expanded_url": "http:\/\/bbc.in\/2kwKPxb",
			"display_url": "bbc.in\/2kwKPxb",
			"indices": [46, 69]
		}]
	},
	"source": "\u003ca href=\"http:\/\/www.socialflow.com\" rel=\"nofollow\"\u003eSocialFlow\u003c\/a\u003e",
	"in_reply_to_status_id": null,
	"in_reply_to_status_id_str": null,
	"in_reply_to_user_id": null,
	"in_reply_to_user_id_str": null,
	"in_reply_to_screen_name": null,
	"user": {
		"id": 742143,
		"id_str": "742143",
		"name": "BBC News (World)",
		"screen_name": "BBCWorld",
		"location": "London, UK",
		"description": "News, features and analysis from the World's newsroom. Breaking news, follow @BBCBreaking. UK news, @BBCNews. Latest sports news @BBCSport",
		"url": "https:\/\/t.co\/7NEgoMwJy3",
		"entities": {
			"url": {
				"urls": [{
					"url": "https:\/\/t.co\/7NEgoMwJy3",
					"expanded_url": "http:\/\/www.bbc.com\/news",
					"display_url": "bbc.com\/news",
					"indices": [0, 23]
				}]
			},
			"description": {
				"urls": []
			}
		},
		"protected": false,
		"followers_count": 17856120,
		"friends_count": 76,
		"listed_count": 102047,
		"created_at": "Thu Feb 01 07:44:29 +0000 2007",
		"favourites_count": 5,
		"utc_offset": 0,
		"time_zone": "London",
		"geo_enabled": false,
		"verified": true,
		"statuses_count": 249668,
		"lang": "en",
		"contributors_enabled": false,
		"is_translator": false,
		"is_translation_enabled": true,
		"profile_background_color": "FFFFFF",
		"profile_background_image_url": "http:\/\/pbs.twimg.com\/profile_background_images\/459295591915204608\/P0byaGJj.jpeg",
		"profile_background_image_url_https": "https:\/\/pbs.twimg.com\/profile_background_images\/459295591915204608\/P0byaGJj.jpeg",
		"profile_background_tile": false,
		"profile_image_url": "http:\/\/pbs.twimg.com\/profile_images\/694449140269518848\/57ZmXva0_normal.jpg",
		"profile_image_url_https": "https:\/\/pbs.twimg.com\/profile_images\/694449140269518848\/57ZmXva0_normal.jpg",
		"profile_banner_url": "https:\/\/pbs.twimg.com\/profile_banners\/742143\/1485172490",
		"profile_link_color": "1F527B",
		"profile_sidebar_border_color": "FFFFFF",
		"profile_sidebar_fill_color": "FFFFFF",
		"profile_text_color": "5A5A5A",
		"profile_use_background_image": true,
		"has_extended_profile": false,
		"default_profile": false,
		"default_profile_image": false,
		"following": true,
		"follow_request_sent": false,
		"notifications": false,
		"translator_type": "none"
	},
	"geo": null,
	"coordinates": null,
	"place": null,
	"contributors": null,
	"is_quote_status": false,
	"retweet_count": 12,
	"favorite_count": 22,
	"favorited": false,
	"retweeted": false,
	"possibly_sensitive": false,
	"possibly_sensitive_appealable": false,
	"lang": "en"
}]`
	tweets, err := TestTwitter.ExtractTweets([]byte(input), 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(tweets) != 1 || tweets[0].User.Name != "BBC News (World)" || tweets[0].Text != `Cambodia opposition leader Sam Rainsy resigns https://t.co/FgXF8S97Qt` {
		t.Fatal(tweets)
	}
	tweets, err = TestTwitter.ExtractTweets([]byte(input), 1, 1)
	if err != nil || len(tweets) != 0 {
		t.Fatal(err, tweets)
	}
}
