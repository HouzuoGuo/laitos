package inet

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/misc"
)

type OAuthSigner struct {
	ConsumerKey       string
	ConsumerSecret    string
	AccessToken       string
	AccessTokenSecret string
}

func (a *OAuthSigner) SetAuthorizationHeader(req *http.Request) error {
	nounce := make([]byte, 32)
	if _, err := rand.Read(nounce); err != nil {
		return err
	}
	// Start from a bunch of standard parameter keys...
	oauthParams := map[string]string{
		"oauth_consumer_key":     a.ConsumerKey,
		"oauth_nonce":            base64.StdEncoding.EncodeToString(nounce),
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        strconv.FormatInt(time.Now().Unix(), 10),
		"oauth_token":            a.AccessToken,
		"oauth_version":          "1.0",
	}
	// Collect parameters from URL query string
	for key, value := range req.URL.Query() {
		oauthParams[key] = value[0]
	}
	// Collect parameters from form submission (up to 8MB in size)
	if req.Body != nil && req.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
		body, err := misc.ReadAllUpTo(req.Body, 8*1024*1024)
		if err != nil {
			return err
		}
		// Pick up individual key=value submissions
		formKeyValue, err := url.ParseQuery(string(body))
		if err != nil {
			return err
		}
		for key, value := range formKeyValue {
			oauthParams[key] = value[0]
		}
		// Allow other users of the HTTP request to re-process the request body
		req.Body = ioutil.NopCloser(bytes.NewReader(body))
	}
	// Calculate the request resource URI, which will become part of the content to sign.
	reqHost := strings.ToLower(req.URL.Host)
	if hostWithPort := strings.Split(reqHost, ":"); len(hostWithPort) == 2 && (hostWithPort[1] == "80" || hostWithPort[1] == "443") {
		// Do not repeat the port number of a well known scheme (HTTP or HTTPS)
		reqHost = hostWithPort[0]
	}
	// EscapedPath gets the resource name without its query string
	resourceURI := fmt.Sprintf("%v://%v%v", strings.ToLower(req.URL.Scheme), reqHost, req.URL.EscapedPath())
	contentToSign := fmt.Sprintf("%s&%s&%s",
		strings.ToUpper(req.Method),
		url.QueryEscape(resourceURI),
		url.QueryEscape(strings.Join(sortEscapeKeyValues(oauthParams), "&")))

	// Calculate the HMAC signature from the info dissected so far and put everything into OAuth header
	hmacHash := hmac.New(sha1.New, []byte(fmt.Sprintf("%s&%s", a.ConsumerSecret, a.AccessTokenSecret)))
	if _, err := hmacHash.Write([]byte(contentToSign)); err != nil {
		return err
	}
	hmacSignature := hmacHash.Sum(nil)
	oauthParams["oauth_signature"] = base64.StdEncoding.EncodeToString(hmacSignature)
	req.Header.Set("Authorization", "OAuth "+strings.Join(sortEscapeKeyValues(oauthParams), ", "))
	return nil
}

func sortEscapeKeyValues(kv map[string]string) []string {
	// Escape both key and value
	escapedKV := map[string]string{}
	for key, value := range kv {
		escapedKV[url.QueryEscape(key)] = url.QueryEscape(value)
	}
	// Sort keys in alphabetical order
	sortedKeys := make([]string, 0, len(kv))
	for key := range escapedKV {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)
	// Return k1=v1, k2=v2, etc
	ret := make([]string, 0, len(escapedKV))
	for _, key := range sortedKeys {
		ret = append(ret, fmt.Sprintf("%s=%s", key, escapedKV[key]))
	}
	return ret
}
