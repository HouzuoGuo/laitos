package inet

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/aws/aws-xray-sdk-go/xray"
)

// Define properties for an HTTP request for DoHTTP function.
type HTTPRequest struct {
	TimeoutSec  int                       // Read timeout for response (default to 30)
	Method      string                    // HTTP method (default to GET)
	Header      http.Header               // Additional request header (default to nil)
	ContentType string                    // Content type header (default to "application/x-www-form-urlencoded")
	Body        io.Reader                 // HTTPRequest body (default to nil)
	RequestFunc func(*http.Request) error // Manipulate the HTTP request at will (default to nil)
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

// doHTTPRequestUsingClient makes an HTTP request via the input HTTP client.Placeholders in the URL template must always use %s.
func doHTTPRequestUsingClient(ctx context.Context, client *http.Client, reqParam HTTPRequest, urlTemplate string, urlValues ...interface{}) (resp HTTPResponse, err error) {
	reqParam.FillBlanks()
	// Encode values in URL path
	encodedURLValues := make([]interface{}, len(urlValues))
	for i, val := range urlValues {
		encodedURLValues[i] = url.QueryEscape(fmt.Sprint(val))
	}
	fullURL := fmt.Sprintf(urlTemplate, encodedURLValues...)
	req, err := http.NewRequestWithContext(ctx, reqParam.Method, fullURL, reqParam.Body)
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
	if len(reqParam.Header) > 0 {
		if contentType := reqParam.Header.Get("Content-Type"); contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
	}
	client.Timeout = time.Duration(reqParam.TimeoutSec) * time.Second
	defer client.CloseIdleConnections()
	// Send the request away, and retry in case of error.
	for retry := 0; retry < reqParam.MaxRetry; retry++ {
		var httpResp *http.Response
		httpResp, err = client.Do(req)
		if err == nil {
			resp.Body, err = misc.ReadAllUpTo(httpResp.Body, reqParam.MaxBytes)
			resp.Header = httpResp.Header
			resp.StatusCode = httpResp.StatusCode
			httpResp.Body.Close()
			if err == nil && httpResp.StatusCode/400 != 1 && httpResp.StatusCode/500 != 1 {
				// Return the response upon success
				if retry > 0 {
					// Let operator know that this URL endpoint may not be quite reliable
					lalog.DefaultLogger.Info("DoHTTP", urlTemplate, nil, "took %d retries to complete this %s request", retry, reqParam.Method)
				}
				return
			}
		}
		// Retry in case of IO error, 4xx, and 5xx responses.
		time.Sleep(1 * time.Second)
	}
	return
}

// DoHTTP makes an HTTP request and returns its HTTP response. Placeholders in the URL template must always use %s.
func DoHTTP(ctx context.Context, reqParam HTTPRequest, urlTemplate string, urlValues ...interface{}) (resp HTTPResponse, err error) {
	client := &http.Client{}
	// Integrate the decorated handler with AWS x-ray. The crucial x-ray daemon program seems to be only capable of running on AWS compute resources.
	if misc.EnableAWSIntegration && IsAWS() {
		client = xray.Client(client)
	}
	return doHTTPRequestUsingClient(ctx, client, reqParam, urlTemplate, urlValues...)
}
