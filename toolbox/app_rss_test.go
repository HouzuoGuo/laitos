package toolbox

import (
	"context"
	"testing"
)

func TestDeserialiseRSSItems(t *testing.T) {
	sample := `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
 <title>Title of RSS</title>
 <description>Example RSS feed description</description>
 <link>http://www.example.com/rss</link>
 <lastBuildDate>Mon, 06 Sep 2010 00:01:00 +0000 </lastBuildDate>
 <pubDate>Sun, 06 Sep 2009 16:20:00 +0000</pubDate>
 <ttl>3600</ttl>

 <item>
  <title>Entry 1</title>
  <description>Description 1.</description>
  <link>http://www.example.com/1</link>
  <pubDate>Sun, 06 Sep 2009 16:20:00 +0500</pubDate>
 </item>

 <item>
  <title>Entry 2</title>
  <description>Description 2.</description>
  <link>http://www.example.com/2</link>
  <pubDate>06 Sep 2009 16:20 GMT</pubDate>
 </item>

 <item>
  <title>Entry 3</title>
  <description>Description 2.</description>
  <link>http://www.example.com/2</link>
  <pubDate>Sun, 06 Sep 2009 16:20 UTC</pubDate>
 </item>

 <item>
  <title>Entry 4</title>
  <description>Description 2.</description>
  <link>http://www.example.com/2</link>
  <pubDate>06 Sep 09 16:20 +0000</pubDate>
 </item>

</channel>
</rss>`

	entries, err := DeserialiseRSSItems([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("%+v", entries)
	}
}

func TestDownloadRSSFeeds(t *testing.T) {
	feeds, err := DownloadRSSFeeds(context.Background(), 10, "http://feeds.bbci.co.uk/news/rss.xml")
	if err != nil || len(feeds) < 10 {
		t.Fatalf("%+v %+v", err, feeds)
	}
}

func TestRSS_Execute(t *testing.T) {
	rss := RSS{}
	if err := rss.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := rss.SelfTest(); err != nil {
		t.Fatal(err)
	}
	// Break a URL and test again
	rss.Sources[0] = "this url does not exist"
	if err := rss.SelfTest(); err == nil {
		t.Fatal("did not error")
	}
	// The broken URL may not hinder other sources from providing their response
	// Retrieve 10 latest feeds (I don't suppose the built-in sources will send more than 5KB per feed..)
	if ret := rss.Execute(context.Background(), Command{TimeoutSec: 30, Content: ""}); ret.Error != nil ||
		len(ret.Output) < 100 || len(ret.Output) > 50*1024 {
		t.Fatal(ret)
	}
	// Bad number - still retrieve 10 latest feeds
	if ret := rss.Execute(context.Background(), Command{TimeoutSec: 30, Content: "a, b"}); ret.Error != nil ||
		len(ret.Output) < 100 || len(ret.Output) > 50*1024 {
		t.Fatal(ret)
	}
	// Retrieve 5 feeds after skipping the latest 3 feeds
	if ret := rss.Execute(context.Background(), Command{TimeoutSec: 30, Content: "3, 5"}); ret.Error != nil ||
		len(ret.Output) < 100 || len(ret.Output) > 50*1024 {
		t.Fatal(ret)
	}
	// Retrieve plenty of feeds
	if ret := rss.Execute(context.Background(), Command{TimeoutSec: 30, Content: "3, 5000"}); ret.Error != nil ||
		len(ret.Output) < 100 {
		t.Fatal(ret)
	}
}
