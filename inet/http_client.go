package inet

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/HouzuoGuo/laitos/misc"
)

// Define properties for an HTTP request for DoHTTP function.
type HTTPRequest struct {
	TimeoutSec  int                       // Read timeout for response (default to 30)
	Method      string                    // HTTP method (default to GET)
	Header      http.Header               // Additional request header (default to nil)
	ContentType string                    // Content type header (default to "application/x-www-form-urlencoded")
	Body        io.Reader                 // HTTPRequest body (default to nil)
	RequestFunc func(*http.Request) error // Manipulate the HTTP request at will (default to nil)
	InsecureTLS bool                      // InsecureTLS may be turned on to ignore all TLS verification errors from an HTTPS client connection
	MaxBytes    int                       // MaxBytes is the maximum number of bytes of response body to read (default to 4MB)
	MaxRetry    int                       // MaxRetry is the maximum number of attempts to make the same request in case of an IO error, 4xx, or 5xx response (default to 3).
}

// Set blank attributes to their default value.
func (req *HTTPRequest) FillBlanks() {
	if req.TimeoutSec <= 0 {
		req.TimeoutSec = 30
	}
	if req.Method == "" {
		req.Method = "GET"
	}
	if req.ContentType == "" {
		req.ContentType = "application/x-www-form-urlencoded"
	}
	if req.MaxBytes <= 0 {
		req.MaxBytes = 4 * 1048576
	}
	if req.MaxRetry < 1 {
		req.MaxRetry = 3
	}
}

// HTTP response as read by DoHTTP function.
type HTTPResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// If HTTP status is not 2xx, return an error. Otherwise return nil.
func (resp *HTTPResponse) Non2xxToError() error {
	// Avoid showing the entire HTTP (quite likely HTML) response to end-user
	compactBody := resp.Body
	if compactBody == nil {
		compactBody = []byte("<IO error prior to response>")
	} else if len(compactBody) > 256 {
		compactBody = compactBody[:256]
	} else if len(compactBody) == 0 {
		compactBody = []byte("<empty response>")
	}

	if resp.StatusCode/200 != 1 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(compactBody))
	} else {
		return nil
	}
}

// GetBodyUpTo returns response body but only up to the specified number of bytes.
func (resp *HTTPResponse) GetBodyUpTo(nBytes int) []byte {
	if resp.Body == nil {
		return []byte{}
	}
	ret := resp.Body
	if len(resp.Body) > nBytes {
		ret = resp.Body[:nBytes]
	}
	return ret
}

// Generic function for sending an HTTP request. Placeholders in URL template must be "%s".
func DoHTTP(reqParam HTTPRequest, urlTemplate string, urlValues ...interface{}) (resp HTTPResponse, err error) {
	reqParam.FillBlanks()
	// Encode values in URL path
	encodedURLValues := make([]interface{}, len(urlValues))
	for i, val := range urlValues {
		encodedURLValues[i] = url.QueryEscape(fmt.Sprint(val))
	}
	fullURL := fmt.Sprintf(urlTemplate, encodedURLValues...)
	req, err := http.NewRequest(reqParam.Method, fullURL, reqParam.Body)
	if err != nil {
		return
	}
	if reqParam.Header != nil {
		req.Header = reqParam.Header
	}
	// Use the input function to further customise the HTTP request
	if reqParam.RequestFunc != nil {
		if err = reqParam.RequestFunc(req); err != nil {
			return
		}
	}
	req.Header.Set("Content-Type", reqParam.ContentType)
	// Construct HTTP client and optionally disable TLS strict verification
	client := &http.Client{
		Timeout:   time.Duration(reqParam.TimeoutSec) * time.Second,
		Transport: &http.Transport{},
	}
	defer client.CloseIdleConnections()
	if reqParam.InsecureTLS {
		client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	// Send the request away, and retry in case of error.
	for attempt := 0; attempt < reqParam.MaxRetry; attempt++ {
		var httpResp *http.Response
		httpResp, err = client.Do(req)
		if err == nil {
			resp.Body, err = misc.ReadAllUpTo(httpResp.Body, reqParam.MaxBytes)
			resp.Header = httpResp.Header
			resp.StatusCode = httpResp.StatusCode
			httpResp.Body.Close()
			if err == nil && httpResp.StatusCode/400 != 1 && httpResp.StatusCode/500 != 1 {
				// Return the response upon success
				return
			}
		}
		// Retry in case of IO error, 4xx, and 5xx responses.
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	return
}
