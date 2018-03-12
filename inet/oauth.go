package inet

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// FIXME: clean this up (howard)
/*
The source code in this file take heavy inspiration from "oauth1" project -
https://github.com/dghubble/oauth1 - authored by "dghubble", carrying the following license text:

Copyright (c) 2015 Dalton Hubble

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

// PercentEncode percent encodes a string according to RFC 3986 2.1.
func PercentEncode(input string) string {
	var buf bytes.Buffer
	for _, b := range []byte(input) {
		if 'A' <= b && b <= 'Z' || 'a' <= b && b <= 'z' || '0' <= b && b <= '9' {
			buf.WriteByte(b)
		} else if b == '-' || b == '.' || b == '_' || b == '~' {
			buf.WriteByte(b)
		} else {
			buf.Write([]byte(fmt.Sprintf("%%%02X", b)))
		}
	}
	return buf.String()
}

type OAuthHeader struct {
	ConsumerKey       string
	ConsumerSecret    string
	AccessToken       string
	AccessTokenSecret string
}

// Sign creates a concatenated consumer and token secret key and calculates
// the HMAC digest of the message. Returns the base64 encoded digest bytes.
func (a *OAuthHeader) Sign(tokenSecret, message string) (string, error) {
	signingKey := strings.Join([]string{a.ConsumerSecret, tokenSecret}, "&")
	mac := hmac.New(sha1.New, []byte(signingKey))
	mac.Write([]byte(message))
	signatureBytes := mac.Sum(nil)
	return base64.StdEncoding.EncodeToString(signatureBytes), nil
}

// setRequestAuthHeader sets the OAuth1 header for making authenticated
// requests with an AccessToken (token credential) according to RFC 5849 3.1.
func (a *OAuthHeader) SetRequestAuthHeader(req *http.Request) error {
	nounce := make([]byte, 32)
	rand.Read(nounce)

	oauthParams := map[string]string{
		"oauth_consumer_key":     a.ConsumerKey,
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        strconv.FormatInt(time.Now().Unix(), 10),
		"oauth_nonce":            base64.StdEncoding.EncodeToString(nounce),
		"oauth_version":          "1.0",
	}
	oauthParams["oauth_token"] = a.AccessToken
	params, err := collectParameters(req, oauthParams)
	if err != nil {
		return err
	}
	signatureBase := signatureBase(req, params)
	signature, err := a.Sign(a.AccessTokenSecret, signatureBase)
	if err != nil {
		return err
	}
	oauthParams["oauth_signature"] = signature
	pairs := sortParameters(encodeParameters(oauthParams), `%s="%s"`)
	req.Header.Set("Authorization", "OAuth "+strings.Join(pairs, ", "))
	return nil
}

// encodeParameters percent encodes parameter keys and values according to
// RFC5849 3.6 and RFC3986 2.1 and returns a new map.
func encodeParameters(params map[string]string) map[string]string {
	encoded := map[string]string{}
	for key, value := range params {
		encoded[PercentEncode(key)] = PercentEncode(value)
	}
	return encoded
}

// sortParameters sorts parameters by key and returns a slice of key/value
// pairs formatted with the given format string (e.g. "%s=%s").
func sortParameters(params map[string]string, format string) []string {
	// sort by key
	keys := make([]string, len(params))
	i := 0
	for key := range params {
		keys[i] = key
		i++
	}
	sort.Strings(keys)
	// parameter join
	pairs := make([]string, len(params))
	for i, key := range keys {
		pairs[i] = fmt.Sprintf(format, key, params[key])
	}
	return pairs
}

// collectParameters collects request parameters from the request query, OAuth
// parameters (which should exclude oauth_signature), and the request body
// provided the body is single part, form encoded, and the form content type
// header is set. The returned map of collected parameter keys and values
// follow RFC 5849 3.4.1.3, except duplicate parameters are not supported.
func collectParameters(req *http.Request, oauthParams map[string]string) (map[string]string, error) {
	// add oauth, query, and body parameters into params
	params := map[string]string{}
	for key, value := range req.URL.Query() {
		// most backends do not accept duplicate query keys
		params[key] = value[0]
	}
	if req.Body != nil && req.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
		// reads data to a []byte, draining req.Body
		b, err := misc.ReadAllUpTo(req.Body, 32*1048576)
		if err != nil {
			return nil, err
		}
		values, err := url.ParseQuery(string(b))
		if err != nil {
			return nil, err
		}
		for key, value := range values {
			// not supporting params with duplicate keys
			params[key] = value[0]
		}
		// reinitialize Body with ReadCloser over the []byte
		req.Body = ioutil.NopCloser(bytes.NewReader(b))
	}
	for key, value := range oauthParams {
		params[key] = value
	}
	return params, nil
}

// signatureBase combines the uppercase request method, percent encoded base
// string URI, and normalizes the request parameters int a parameter string.
// Returns the OAuth1 signature base string according to RFC5849 3.4.1.
func signatureBase(req *http.Request, params map[string]string) string {
	method := strings.ToUpper(req.Method)
	baseURL := baseURI(req)
	parameterString := strings.Join(sortParameters(encodeParameters(params), "%s=%s"), "&")
	// signature base string constructed accoding to 3.4.1.1
	baseParts := []string{method, PercentEncode(baseURL), PercentEncode(parameterString)}
	return strings.Join(baseParts, "&")
}

// baseURI returns the base string URI of a request according to RFC 5849
// 3.4.1.2. The scheme and host are lowercased, the port is dropped if it
// is 80 or 443, and the path minus query parameters is included.
func baseURI(req *http.Request) string {
	scheme := strings.ToLower(req.URL.Scheme)
	host := strings.ToLower(req.URL.Host)
	if hostPort := strings.Split(host, ":"); len(hostPort) == 2 && (hostPort[1] == "80" || hostPort[1] == "443") {
		host = hostPort[0]
	}
	// TODO: use req.URL.EscapedPath() once Go 1.5 is more generally adopted
	// For now, hacky workaround accomplishes the same internal escaping mode
	// escape(u.Path, encodePath) for proper compliance with the OAuth1 spec.
	path := req.URL.Path
	if path != "" {
		path = strings.Split(req.URL.RequestURI(), "?")[0]
	}
	return fmt.Sprintf("%v://%v%v", scheme, host, path)
}
