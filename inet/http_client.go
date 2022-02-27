package inet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/aws/aws-xray-sdk-go/xray"
)

var (
	// NeutralRecursiveResolvers is a list of public recursive DNS resolvers
	// that provide genuine answers without discrimination or filtering.
	// These resolvers must support both TCP and UDP.
	NeutralDNSResolverAddrs = []string{
		// Cloudflare (https://1.1.1.1/dns/)
		"1.1.1.1:53",
		"1.0.0.1:53",
		// Google public DNS (https://developers.google.com/speed/public-dns)
		"8.8.8.8:53",
		"8.8.4.4:53",
		// Hurricane electric (https://dns.he.net/)
		"74.82.42.42:53",
		// 20220227 - Do not use OpenNIC any longer, the IP addresses are not stable. OpenNIC (https://www.opennic.org/) "185.121.177.177:53", "169.239.202.202:53",
		// Quad9 without blocklists (https://www.quad9.net/support/faq/)
		"9.9.9.10:53",
		"149.112.112.10:53",
	}

	// NeutralRecursiveResolver is a DNS resolver that provides genuine answers
	// without discrimination or filtering. This is often useful for resolving
	// names downloaded from various blacklist projects.
	NeutralRecursiveResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (conn net.Conn, e error) {
			// Obey the network protocol choice (udp, tcp), but insist on using
			// one of the built-in resolver addresses.
			resolverAddr := NeutralDNSResolverAddrs[rand.Intn(len(NeutralDNSResolverAddrs))]
			var d net.Dialer
			return d.DialContext(ctx, network, resolverAddr)
		},
	}
)

// HTTPRequest defines all of the parameters necessary for making an outgoing HTTP request using the DoHTTP function.
type HTTPRequest struct {
	// TimeoutSec is the timeout of the execution of the entire HTTP request, defaults to 30 seconds.
	TimeoutSec int
	// Method is the HTTP method name, defaults to "GET".
	Method string
	// Header is the collection of additional request headers, defaults to nil.
	Header http.Header
	// ContentType is the request content type, defaults to "application/x-www-form-urlencoded".
	ContentType string
	// Body is the HTTP request body, defaults to nil.
	Body io.Reader
	// RequestFunc is invoked shortly before executing the HTTP request, allowing caller to further customise the request, defaults to nil.
	RequestFunc func(*http.Request) error
	// MaxBytes is the maximum size of response body to read, defaults to 4MB.
	MaxBytes int
	// MaxRetry is the maximum number of retries to make in case of an IO error, 4xx, or 5xx response, defaults to 3.
	MaxRetry int
	// UseNeutralDNSResolver instructs the HTTP client to use the neutral & recursive public DNS resolver instead of the default resolver of the system.
	UseNeutralDNSResolver bool
}

// FillBlanks gives sets the parameters of the HTTP request using sensible default values.
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

// HTTPResponse encapsulates the response code, header, and response body in its entirety.
type HTTPResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// Non2xxToError returns an error only if the HTTP response status is not 2xx.
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
func doHTTPRequestUsingClient(ctx context.Context, client *http.Client, reqParam HTTPRequest, urlTemplate string, urlValues ...interface{}) (HTTPResponse, error) {
	defer client.CloseIdleConnections()
	// Use context to handle the timeout of the entire lifespan of this HTTP request
	reqParam.FillBlanks()
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, time.Duration(reqParam.TimeoutSec)*time.Second)
	defer timeoutCancel()
	// Use the neutral & public recursive DNS resolver if desired
	if reqParam.UseNeutralDNSResolver {
		if client.Transport == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyFromEnvironment}
		}
		switch transport := client.Transport.(type) {
		case *http.Transport:
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer := &net.Dialer{
					Resolver:  NeutralRecursiveResolver,
					DualStack: true,
				}
				return dialer.DialContext(ctx, network, addr)
			}
		default:
			// The transport likely does not support DialContext.
		}
	}
	// Encode values in URL path
	encodedURLValues := make([]interface{}, len(urlValues))
	for i, val := range urlValues {
		encodedURLValues[i] = url.QueryEscape(fmt.Sprint(val))
	}
	fullURL := fmt.Sprintf(urlTemplate, encodedURLValues...)
	// Retain a copy of request body for retry
	reqBodyCopy := new(bytes.Buffer)
	var lastHTTPErr error
	var lastResponse HTTPResponse
	// Send the request away, and retry in case of error.
	for retry := 0; retry < reqParam.MaxRetry; retry++ {
		var reqBodyReader io.Reader
		if reqParam.Body != nil {
			if retry == 0 {
				// Retain a copy of the request body in memory
				reqBodyReader = io.TeeReader(reqParam.Body, reqBodyCopy)
			} else {
				// Use the in-memory copy of request body from now as the original stream has already been drained
				reqBodyReader = bytes.NewReader(reqBodyCopy.Bytes())
			}
		}
		req, err := http.NewRequestWithContext(timeoutCtx, reqParam.Method, fullURL, reqBodyReader)
		if err != nil {
			return HTTPResponse{}, err
		}
		if reqParam.Header != nil {
			req.Header = reqParam.Header
		}
		// Use the input function to further customise the HTTP request
		if reqParam.RequestFunc != nil {
			if err := reqParam.RequestFunc(req); err != nil {
				return HTTPResponse{}, err
			}
		}
		req.Header.Set("Content-Type", reqParam.ContentType)
		if len(reqParam.Header) > 0 {
			if contentType := reqParam.Header.Get("Content-Type"); contentType != "" {
				req.Header.Set("Content-Type", contentType)
			}
		}
		var httpResp *http.Response
		httpResp, lastHTTPErr = client.Do(req)
		if lastHTTPErr == nil {
			lastResponse = HTTPResponse{
				Header:     httpResp.Header,
				StatusCode: httpResp.StatusCode,
			}
			lastResponse.Body, lastHTTPErr = misc.ReadAllUpTo(httpResp.Body, reqParam.MaxBytes)
			lalog.DefaultLogger.MaybeMinorError(httpResp.Body.Close())
			if lastHTTPErr == nil && httpResp.StatusCode/400 != 1 && httpResp.StatusCode/500 != 1 {
				// Return the response upon success
				if retry > 0 {
					// Let operator know that this URL endpoint may not be quite reliable
					lalog.DefaultLogger.Info("DoHTTP", urlTemplate, nil, "took %d retries to complete this %s request", retry, reqParam.Method)
				}
				return lastResponse, nil
			}
		}
		// Retry in case of IO error, 4xx, and 5xx responses.
		time.Sleep(1 * time.Second)
	}
	// Having exhausted all attempts, return the status code, body, etc, that belong to the latest response.
	return lastResponse, lastHTTPErr
}

// DoHTTP makes an HTTP request and returns its HTTP response. Placeholders in the URL template must always use %s.
func DoHTTP(ctx context.Context, reqParam HTTPRequest, urlTemplate string, urlValues ...interface{}) (resp HTTPResponse, err error) {
	client := &http.Client{}
	// Integrate the decorated handler with AWS x-ray. Be aware that the x-ray daemon program mandatory for collecting traces only runs on AWS EC2.
	// The x-ray library gracefully does nothing when it runs on non-EC2 instances.
	if misc.EnableAWSIntegration && IsAWS() {
		client = xray.Client(client)
	}
	return doHTTPRequestUsingClient(ctx, client, reqParam, urlTemplate, urlValues...)
}
