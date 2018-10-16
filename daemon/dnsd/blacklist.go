package dnsd

import (
	"strings"
	"sync"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
)

// HostsFileURLs is a collection of URLs where up-to-date ad/malware/spyware blacklist hosts files are published.
var HostsFileURLs = []string{
	"http://winhelp2002.mvps.org/hosts.txt",
	"http://pgl.yoyo.org/adservers/serverlist.php?hostformat=hosts&showintro=0&mimetype=plaintext",
	"http://www.malwaredomainlist.com/hostslist/hosts.txt",
	"http://someonewhocares.org/hosts/hosts",
}

/*
Whitelist is an array of domain names that often appear in black lists, but cause inconvenience when blocked. These
names are removed from downloaded black lists.
*/
var Whitelist = []string{
	/*
		2018-06-24 - youtube app on iPhone fails to save watch history, some sources suggest that this domain name is
		the culprit.
	*/
	"s.youtube.com",
}

/*
DownloadAllBlacklists attempts to download all hosts files and return combined list of domain names to block.
The special cases of white listed names are removed from return value.
*/
func DownloadAllBlacklists(logger lalog.Logger) []string {
	wg := new(sync.WaitGroup)
	wg.Add(len(HostsFileURLs))

	// Download all lists in parallel
	lists := make([][]string, len(HostsFileURLs))
	for i, url := range HostsFileURLs {
		go func(i int, url string) {
			resp, err := inet.DoHTTP(inet.HTTPRequest{TimeoutSec: BlacklistDownloadTimeoutSec}, url)
			names := ExtractNamesFromHostsContent(string(resp.Body))
			logger.Info("DownloadAllBlacklists", url, err, "downloaded %d names, please obey the license in which the list author publishes the data.", len(names))
			lists[i] = ExtractNamesFromHostsContent(string(resp.Body))
			defer wg.Done()
		}(i, url)
	}
	wg.Wait()
	// Calculate unique set of domain names
	set := map[string]struct{}{}
	for _, list := range lists {
		for _, str := range list {
			set[str] = struct{}{}
		}
	}
	// Remove white listed names
	for _, toRemove := range Whitelist {
		delete(set, toRemove)
	}

	ret := make([]string, 0, len(set))
	for str := range set {
		ret = append(ret, str)
	}
	logger.Info("DownloadAllBlacklists", "", nil, "downloaded %d unique names in total", len(ret))
	return ret
}

/*
ExtractNamesFromHostsContent extracts domain names from hosts file content. It will understand and skip comments and
empty lines.
*/
func ExtractNamesFromHostsContent(content string) []string {
	ret := make([]string, 0, 16384)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			// Skip blank and comments
			continue
		}
		// Find the second field
		space := strings.IndexRune(line, ' ')
		if space == -1 {
			// Skip malformed line
			continue
		}
		line = strings.TrimSpace(line[space:])
		nameEnd := strings.IndexRune(line, '#')
		// Name may be followed by a comment
		if nameEnd == -1 {
			nameEnd = len(line)
		}
		// Extract the name itself. Matching of black list name always takes place in lower case.
		aName := strings.ToLower(strings.TrimSpace(line[:nameEnd]))
		if aName == "" || strings.HasSuffix(aName, "localhost") || strings.HasSuffix(aName, "localdomain") || len(aName) < 4 {
			// Skip empty names, local names, and overly short names
			continue
		}
		ret = append(ret, aName)
		if len(ret) > 50000 {
			// Avoid taking in too many names
			break
		}
	}
	return ret
}
