package toolbox

import (
	"encoding/xml"
	"fmt"
	"time"
)

// DefaultRSSSources is a list of RSS of news headlines published by major news agencies around the world.
var DefaultRSSSources = []string{
	// "Top stories" from Australia
	"http://www.abc.net.au/news/feed/45910/rss.xml",
	// "News front page" from BBC
	"http://feeds.bbci.co.uk/news/rss.xml",
	// "Top news" from Reuters
	"http://feeds.reuters.com/reuters/topNews",
	// "Top news" from CNBC
	"https://www.cnbc.com/id/100003114/device/rss/rss.html",
	// "Top stories" from CNN
	"http://rss.cnn.com/rss/edition.rss",
	// "Homepage" from Jerusalem Post
	"http://www.jpost.com/Rss/RssFeedsFrontPage.aspx",
}

type RSS struct {
	/*
		Sources are URLs pointing toward RSS feeds in XML format. If left unspecified, the built-in list of sources that
		point to news headlines will be used.
	*/
	Sources []string `json:"Sources"`
}

func (rss *RSS) IsConfigured() bool {
	// Even if RSS sources are not specified, the built-in list will continue to work.
	return true
}

func (rss *RSS) SelfTest() error {
	//
}

func (rss *RSS) Initialise() error {
	// If RSS sources are not specified, use the default sources that are world news headlines.
	if rss.Sources == nil || len(rss.Sources) == 0 {
		rss.Sources = DefaultRSSSources
	}
}

func (rss *RSS) Trigger() Trigger {
	return ".r"
}

func (rss *RSS) Execute(cmd Command) *Result {

}

// RSSRoot is the root element in an RSS XML document.
type RSSRoot struct {
	Channel RSSChannel `xml:"channel"`
}

// RSSChannel represents an information channel in RSS XML document.
type RSSChannel struct {
	Items []RSSItem `xml:"item"`
}

// RSSItem represents a news item in RSS XML document.
type RSSItem struct {
	Title       string     `xml:"title"`
	Description string     `xml:"description"`
	PubDate     RSSPubDate `xml:"pubDate"`
}

// RSSPubDate represents a publication date/time stamp in RSS XML document.
type RSSPubDate struct {
	time.Time
}

func (pubDate *RSSPubDate) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var pubDateStr string
	if err := d.DecodeElement(&pubDateStr, &start); err != nil {
		return err
	}
	// Publication date is written in RFC822 format, however the year may be written in either two or four digits.
	if parsed, err := time.Parse(time.RFC822, pubDateStr); err == nil {
		*pubDate = RSSPubDate{parsed}
	} else if parsed, err := time.Parse(`02 Jan 2006 15:04 MST`, pubDateStr); err == nil {
		*pubDate = RSSPubDate{parsed}
	} else {
		return fmt.Errorf("RSSPubDate.UnmarshalXML: failed to interpret publication date \"%s\"", err)
	}
	return nil
}
