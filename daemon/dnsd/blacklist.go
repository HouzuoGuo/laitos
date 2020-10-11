package dnsd

import (
	"context"
	"strings"
	"sync"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
)

const (
	/*
		MaxNameEntriesToExtract is the maximum number of entries to be extracted from one instance of hosts file.
		The limit prevents an exceedingly long third party host file from taking too much memory.
	*/
	MaxNameEntriesToExtract = 50000
)

// HostsFileURLs is a collection of URLs where up-to-date ad/malware/spyware blacklist hosts files are published.
var HostsFileURLs = []string{
	"http://winhelp2002.mvps.org/hosts.txt",
	"http://pgl.yoyo.org/adservers/serverlist.php?hostformat=hosts&showintro=0&mimetype=plaintext",
	"http://www.malwaredomainlist.com/hostslist/hosts.txt",
	"http://someonewhocares.org/hosts/hosts",
	"https://hosts.oisd.nl/light",
	"https://raw.githubusercontent.com/blocklistproject/Lists/master/ransomware.txt",
	"https://raw.githubusercontent.com/blocklistproject/Lists/master/scam.txt",
	"https://raw.githubusercontent.com/blocklistproject/Lists/master/tracking.txt",
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
	/*
		2019-05-23 - facebook app and instagram app take ~1 minute to load due to blocked connectivity to their graph
		API domains.
	*/
	"graph.facebook.com",
	"graph.instagram.com",
	/*
		"Common whitelisted domains"(https://discourse.pi-hole.net/t/commonly-whitelisted-domains/212) as seen on 2020-09-08
	*/
	"106c06cd218b007d-b1e8a1331f68446599e96a4b46a050f5.ams.plex.services", "79423.analytics.edgekey.net", "android.clients.google.com", "api.facebook.com", "app.plex.tv", "appleid.apple.com",
	"apps.skype.com", "appspot-preview.l.google.com", "apresolve.spotify.com", "apt.sonarr.tv", "assets.adobedtm.com", "attestation.xboxlive.com", "b-api.facebook.com", "b-graph.facebook.com",
	"bigzipfiles.facebook.com", "c.s-microsoft.com", "captive.apple.com", "cdn.fbsbx.com", "cdn.vidible.tv", "cert.mgt.xboxlive.com", "chevrolet.com", "client-s.gateway.messenger.live.com",
	"clientconfig.passport.net", "clients2.google.com", "clients3.google.com", "clients4.google.com", "code.bildstatic.de", "connect.facebook.com", "connectivitycheck.android.com", "connectivitycheck.gstatic.com",
	"cpms.spop10.ams.plex.bz", "cpms35.spop10.ams.plex.bz", "creative.ak.fbcdn.net", "ctldl.windowsupdate.com", "dashboard.plex.tv", "de.ioam.de", "def-vef.xboxlive.com", "delivery.vidible.tv",
	"dev.virtualearth.net", "device.auth.xboxlive.com", "directvapplications.hb.omtrdc.net", "directvnow.com", "displaycatalog.mp.microsoft.com", "dl.delivery.mp.microsoft.com", "dl.dropboxusercontent.com",
	"dl.google.com", "download.sonarr.tv", "ecn.dev.virtualearth.net", "edge-mqtt.facebook.com", "edge.api.brightcove.com", "eds.xboxlive.com", "entitlement.auth.adobe.com", "external-lhr0-1.xx.fbcdn.net",
	"external-lhr1-1.xx.fbcdn.net", "external-lhr10-1.xx.fbcdn.net", "external-lhr2-1.xx.fbcdn.net", "external-lhr3-1.xx.fbcdn.net", "external-lhr4-1.xx.fbcdn.net", "external-lhr5-1.xx.fbcdn.net", "external-lhr6-1.xx.fbcdn.net",
	"external-lhr7-1.xx.fbcdn.net", "external-lhr8-1.xx.fbcdn.net", "external-lhr9-1.xx.fbcdn.net", "fb.me", "fbcdn-creative-a.akamaihd.net", "fe3.delivery.dsp.mp.microsoft.com.nsatc.net", "firestore.googleapis.com",
	"forums.sonarr.tv", "fpdownload.adobe.com", "g.live.com", "geo-prod.do.dsp.mp.microsoft.com", "gfwsl.geforce.com", "googleapis.l.google.com", "graph.facebook.com", "gravatar.com", "gsp1.apple.com", "help.ui.xboxlive.com",
	"i.s-microsoft.com", "imagesak.secureserver.net", "img.vidible.tv", "ipv6.msftncsi.com", "itunes.apple.com", "js.maxmind.com", "json.bild.de", "l.facebook.com", "licensing.xboxlive.com", "livepassdl.conviva.com",
	"login.live.com", "login.microsoftonline.com", "m.hotmail.com", "m.weeklyad.target.com", "meta-db-worker02.pop.ric.plex.bz", "meta.plex.bz", "meta.plex.tv", "metrics.plex.tv", "mqtt.c10r.facebook.com",
	"msftncsi.com", "nexus.ensighten.com", "nine.plugins.plexapp.com", "node.plexapp.com", "notify.xboxlive.com", "ns1.dropbox.com", "ns2.dropbox.com", "o1.email.plex.tv", "o2.sg0.plex.tv", "officeclient.microsoft.com",
	"outlook.office365.com", "placehold.it", "placeholdit.imgix.net", "plex.tv", "portal.fb.com", "pricelist.skype.com", "prod.telemetry.ros.rockstargames.com", "products.office.com", "proxy.plex.bz",
	"proxy.plex.tv", "proxy02.pop.ord.plex.bz", "pubsub.plex.bz", "pubsub.plex.tv", "reminders-pa.googleapis.com", "s.gateway.messenger.live.com", "s.marketwatch.com", "s.mzstatic.com", "s.youtube.com",
	"s.zkcdn.net", "sa.symcb.com", "scontent-lhr3-1.xx.fbcdn.net", "scontent.fgdl5-1.fna.fbcdn.net", "scontent.xx.fbcdn.net", "script.ioam.de", "services.sonarr.tv", "settings-win.data.microsoft.com",
	"skyhook.sonarr.tv", "sls.update.microsoft.com.akadns.net", "spclient.wg.spotify.com", "ssl.google-analytics.com", "staging.plex.tv", "star-mini.c10r.facebook.com", "star.c10r.facebook.com", "status.plex.tv",
	"s1.symcb.com", "s2.symcb.com", "s3.symcb.com", "s4.symcb.com", "s5.symcb.com", "t0.ssl.ak.dynamic.tiles.virtualearth.net", "t0.ssl.ak.tiles.virtualearth.net", "tags.tiqcdn.com", "themoviedb.com", "thetvdb.com",
	"title.auth.xboxlive.com", "title.mgt.xboxlive.com", "tracking-protection.cdn.mozilla.net", "tracking.epicgames.com", "tvdb2.plex.tv", "tvthemes.plexapp.com", "tvthemes.plexapp.com.cdn.cloudflare.net", "ui.skype.com",
	"upload.facebook.com", "v10.events.data.microsoft.com", "v10.vortex-win.data.microsoft.com", "v20.events.data.microsoft.com", "video-stats.l.google.com", "videos.vidible.tv", "weeklyad.target.com",
	"weeklyad.target.com.edgesuite.net", "widget-cdn.rpxnow.com", "www.apple.com", "www.appleiphonecell.com", "www.asadcdn.com", "www.google-analytics.com", "www.msftncsi.com", "www.plex.tv", "www.xboxlive.com",
	"xbox.ipv6.microsoft.com", "xboxexperiencesprod.experimentation.xboxlive.com", "xflight.xboxlive.com", "xkms.xboxlive.com", "xsts.auth.xboxlive.com",
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
			resp, err := inet.DoHTTP(context.Background(), inet.HTTPRequest{TimeoutSec: BlackListDownloadTimeoutSec}, url)
			if err == nil {
				names := ExtractNamesFromHostsContent(string(resp.Body))
				logger.Info("DownloadAllBlacklists", url, err, "downloaded %d names, please obey the license in which the list author publishes the data.", len(names))
				lists[i] = names
			} else {
				logger.Warning("DownloadAllBlacklists", url, err, "failed to download blacklist")
				lists[i] = []string{}
			}
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
ExtractNamesFromHostsContent extracts domain names from hosts file content. It will not return empty lines, comments, and potentially
illegal domain names.
*/
func ExtractNamesFromHostsContent(content string) []string {
	ret := make([]string, 0, 16384)
	for _, line := range strings.Split(content, "\n") {
		if strings.ContainsRune(line, 0) {
			/*
				If attempting to resolve this name that contains NULL byte on Windows, it will unfortunately trigger an
				internal panic in Go's DNS resolution routine.
			*/
			continue
		}
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
		if aName == "" || strings.HasSuffix(aName, "localhost") || strings.HasSuffix(aName, "localdomain") ||
			len(aName) < 4 || len(aName) > 253 {
			// Skip empty names, local names, and overly short names
			// Also, domain name length may not exceed 253 characters according to various technical documents in the public domain.
			continue
		}
		ret = append(ret, aName)
		if len(ret) > MaxNameEntriesToExtract {
			// Avoid taking in too many names
			break
		}
	}
	return ret
}
