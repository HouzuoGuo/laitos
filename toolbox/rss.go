package toolbox

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/inet"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const RSSDownloadTimeoutSec = 10 // RSSDownloadTimeoutSec is the IO timeout used for testing RSS sources.

var (
	// ErrBadRSSParam is the error response for incorrectly entering numeric parameters for retrieving RSS feeds.
	ErrBadRSSParam = errors.New("Example: .s skip# count#")

	// DefaultRSSSources is a list of RSS of news headlines published by major news agencies around the world.
	DefaultRSSSources = []string{
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
)

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
	// Let user know the malfunctioned URLS
	_, err := DownloadRSSFeeds(RSSDownloadTimeoutSec, rss.Sources...)
	if err != nil {
		return fmt.Errorf("RSS.SelfTest: failed to download feeds - %v", err)
	}
	return nil
}

func (rss *RSS) Initialise() error {
	// If RSS sources are not specified, use the default sources that are world news headlines.
	if rss.Sources == nil || len(rss.Sources) == 0 {
		rss.Sources = make([]string, len(DefaultRSSSources))
		copy(rss.Sources, DefaultRSSSources)
	}
	return nil
}

func (rss *RSS) Trigger() Trigger {
	return ".r"
}

func (rss *RSS) Execute(cmd Command) *Result {
	// Input command looks like: skip# count#, find the two numeric parameters among the content
	var skip, count int
	params := RegexTwoNumbers.FindStringSubmatch(cmd.Content)
	if len(params) >= 3 {
		var intErr error
		skip, intErr = strconv.Atoi(params[1])
		if intErr != nil {
			return &Result{Error: ErrBadRSSParam}
		}
		count, intErr = strconv.Atoi(params[2])
		if intErr != nil {
			return &Result{Error: ErrBadRSSParam}
		}
	}
	// If count is not given in the input command, retrieve 10 latest items.
	if count == 0 {
		count = 10
	}
	sortedItems, _ := DownloadRSSFeeds(cmd.TimeoutSec, rss.Sources...)
	if sortedItems == nil || len(sortedItems) == 0 {
		return &Result{Error: errors.New("all RSS sources failed to respond or gave no response")}
	}
	// Skip and limit number of items, but make sure at least one feed will be returned.
	begin := skip
	end := skip + count
	if begin >= len(sortedItems) {
		begin = len(sortedItems) - 1
	}
	if end > len(sortedItems) {
		end = len(sortedItems)
	}
	sortedItems = sortedItems[begin:end]
	// Place an item title and its associated description on each line
	var out bytes.Buffer
	for _, item := range sortedItems {
		out.WriteString(item.Title)
		out.WriteRune('-')
		out.WriteString(item.Description)
		out.WriteRune('\n')
	}
	return &Result{Output: out.String()}
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
	// There is one way to write publication date in the RSS standard, but there are many ways to write it in practice.
	for _, format := range []string{
		// Time zone in letters VS numerals, 2 VS 4 digit year
		`02 Jan 06 15:04 MST`, `02 Jan 06 15:04 -0700`,
		`02 Jan 2006 15:04 MST`, `02 Jan 2006 15:04 -0700`,

		// With additional day of week
		`Mon, 02 Jan 06 15:04 MST`, `Mon, 02 Jan 06 15:04 -0700`,
		`Mon, 02 Jan 2006 15:04 MST`, `Mon, 02 Jan 2006 15:04 -0700`,

		// With additional second
		`02 Jan 06 15:04:05 MST`, `02 Jan 06 15:04:05 -0700`,
		`02 Jan 2006 15:04:05 MST`, `02 Jan 2006 15:04:05 -0700`,

		// With additional day of week & second
		`Mon, 02 Jan 06 15:04:05 MST`, `Mon, 02 Jan 06 15:04:05 -0700`,
		`Mon, 02 Jan 2006 15:04:05 MST`, `Mon, 02 Jan 2006 15:04:05 -0700`} {
		if parsed, err := time.Parse(format, pubDateStr); err == nil {
			*pubDate = RSSPubDate{parsed}
			return nil
		}
	}
	return fmt.Errorf("RSSPubDate.UnmarshalXML: failed to interpret publication date \"%s\"", pubDateStr)
}

/*
DeserialiseRSSItems deserialises RSS feeds from input XML and returns news items among them in their original order.
In case of an error, the error along with an empty array will be returned.
*/
func DeserialiseRSSItems(input []byte) (items []RSSItem, err error) {
	var root RSSRoot
	err = xml.Unmarshal(input, &root)
	items = root.Channel.Items
	if items == nil {
		items = []RSSItem{}
	}
	return
}

// DownloadRSSFeeds downloads RSS feed items from multiple URLs, then orders them from latest to oldest.
func DownloadRSSFeeds(timeoutSec int, xmlURLs ...string) (items []RSSItem, err error) {
	// Memorise errors for each URL
	errs := make(map[string]error)
	items = make([]RSSItem, 0, 10)
	// Prevent concurrent access to the result
	resultMutex := new(sync.Mutex)
	wait := new(sync.WaitGroup)
	for _, aURL := range xmlURLs {
		wait.Add(1)
		go func(aURL string) {
			defer wait.Done()
			// Download feeds XML and deserialise
			resp, err := inet.DoHTTP(inet.HTTPRequest{TimeoutSec: timeoutSec}, strings.Replace(aURL, "%", "%%", -1))
			resultMutex.Lock()
			defer resultMutex.Unlock()
			if err == nil {
				feedItems, feedErr := DeserialiseRSSItems(resp.Body)
				if feedErr == nil {
					items = append(items, feedItems...)
				} else {
					errs[aURL] = err
				}
			} else {
				errs[aURL] = err
			}
		}(aURL)
	}
	wait.Wait()
	// Sort feeds latest to oldest according to publication date
	sort.Slice(items, func(i1, i2 int) bool {
		return items[i1].PubDate.After(items[i2].PubDate.Time)
	})
	// Collect error information
	var errMsg string
	for aURL, err := range errs {
		errMsg += fmt.Sprintf("%s - %v\n", aURL, err)
	}
	if len(errs) > 0 {
		err = errors.New(errMsg)
	}
	return
}
