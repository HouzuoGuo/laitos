package httpd

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/frontend/httpd/api"
	"github.com/HouzuoGuo/laitos/global"
	"github.com/HouzuoGuo/laitos/httpclient"
	"github.com/HouzuoGuo/laitos/ratelimit"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	DirectoryHandlerRateLimitFactor = 10             // 9 times less expensive than the most expensive handler
	RateLimitIntervalSec            = 5              // Rate limit is calculated at 5 seconds interval
	RateLimit404Key                 = "RATELIMIT404" // Fake endpoint name for rate limit on 404 handler
	IOTimeoutSec                    = 120            // IO timeout for both read and write operations
)

// Return true if input character is a forward ot backward slash.
func IsSlash(c rune) bool {
	return c == '\\' || c == '/'
}

// Generic HTTP daemon.
type HTTPD struct {
	ListenAddress    string            `json:"ListenAddress"`    // Network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	ListenPort       int               `json:"ListenPort"`       // Port number to listen on
	TLSCertPath      string            `json:"TLSCertPath"`      // (Optional) serve HTTPS via this certificate
	TLSKeyPath       string            `json:"TLSKeyPath"`       // (Optional) serve HTTPS via this certificate (key)
	BaseRateLimit    int               `json:"BaseRateLimit"`    // How many times in 5 seconds interval the most expensive HTTP handler may be invoked by an IP
	ServeDirectories map[string]string `json:"ServeDirectories"` // Serve directories (value) on prefix paths (key)

	SpecialHandlers map[string]api.HandlerFactory   `json:"-"` // Specialised handlers that implement api.HandlerFactory interface
	AllRoutes       map[string]http.HandlerFunc     `json:"-"` // Aggregate all routes from all handlers
	AllRateLimits   map[string]*ratelimit.RateLimit `json:"-"` // Aggregate all routes and their rate limit counters
	Server          *http.Server                    `json:"-"` // Standard library HTTP server structure
	Processor       *common.CommandProcessor        `json:"-"` // Feature command processor
	Logger          global.Logger                   `json:"-"` // Logger
}

// Return path to HandlerFactory among special handlers that matches the specified type. Primarily used by test case code.
func (httpd *HTTPD) GetHandlerByFactoryType(match api.HandlerFactory) string {
	matchTypeString := reflect.TypeOf(match).String()
	for path, handler := range httpd.SpecialHandlers {
		if reflect.TypeOf(handler).String() == matchTypeString {
			return path
		}
	}
	return ""
}

// Check configuration and initialise internal states.
func (httpd *HTTPD) Initialise() error {
	httpd.Logger = global.Logger{ComponentName: "HTTPD", ComponentID: fmt.Sprintf("%s:%d", httpd.ListenAddress, httpd.ListenPort)}
	if httpd.Processor != nil {
		httpd.Processor.SetLogger(httpd.Logger)
		if errs := httpd.Processor.IsSaneForInternet(); len(errs) > 0 {
			return fmt.Errorf("HTTPD.Initialise: %+v", errs)
		}
	}
	if httpd.ListenAddress == "" {
		return errors.New("HTTPD.Initialise: listen address is empty")
	}
	if httpd.ListenPort < 1 {
		return errors.New("HTTPD.Initialise: listen port must be greater than 0")
	}
	if (httpd.TLSCertPath != "" || httpd.TLSKeyPath != "") && (httpd.TLSCertPath == "" || httpd.TLSKeyPath == "") {
		return errors.New("HTTPD.Initialise: if TLS is to be enabled, both TLS certificate and key path must be present.")
	}
	// Work around Go's inability to serve a handler on / and only /
	httpd.AllRoutes = map[string]http.HandlerFunc{}
	httpd.AllRateLimits = map[string]*ratelimit.RateLimit{}
	// Collect directory handlers
	if httpd.ServeDirectories != nil {
		for urlLocation, dirPath := range httpd.ServeDirectories {
			if len(urlLocation) == 0 {
				continue
			}
			if urlLocation[0] != '/' {
				urlLocation = "/" + urlLocation
			}
			httpd.AllRoutes[urlLocation] = http.StripPrefix(urlLocation, http.FileServer(http.Dir(dirPath))).(http.HandlerFunc)
			httpd.AllRateLimits[urlLocation] = &ratelimit.RateLimit{
				UnitSecs: RateLimitIntervalSec,
				MaxCount: DirectoryHandlerRateLimitFactor * httpd.BaseRateLimit,
				Logger:   httpd.Logger,
			}
		}
	}
	// Collect specialised handlers
	for urlLocation, handler := range httpd.SpecialHandlers {
		fun, err := handler.MakeHandler(httpd.Logger, httpd.Processor)
		if err != nil {
			return err
		}
		httpd.AllRoutes[urlLocation] = fun
		httpd.AllRateLimits[urlLocation] = &ratelimit.RateLimit{
			UnitSecs: RateLimitIntervalSec,
			MaxCount: handler.GetRateLimitFactor() * httpd.BaseRateLimit,
			Logger:   httpd.Logger,
		}
	}
	// There is a rate limit for 404 that does not allow frequent hits
	httpd.AllRateLimits[RateLimit404Key] = &ratelimit.RateLimit{
		UnitSecs: RateLimitIntervalSec,
		MaxCount: httpd.BaseRateLimit,
		Logger:   httpd.Logger,
	}
	// Initialise all rate limits
	for _, limit := range httpd.AllRateLimits {
		limit.Initialise()
	}
	// Install all handlers
	rootHandler := httpd.MakeRootHandlerFunc()
	muxHandlers := http.NewServeMux()
	muxHandlers.HandleFunc("/", rootHandler)
	// Configure server with rather generous and sane defaults
	httpd.Server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", httpd.ListenAddress, httpd.ListenPort),
		Handler:      muxHandlers,
		ReadTimeout:  IOTimeoutSec * time.Second,
		WriteTimeout: IOTimeoutSec * time.Second,
	}
	return nil
}

/*
Create a handler func that serves all of the input routes.
Input routes must use forward slash in URL.
This function exists to work around Go's inability to serve an independent handler on /.
*/
func (httpd *HTTPD) MakeRootHandlerFunc() http.HandlerFunc {
	maxURLFields := 0
	for urlLocation := range httpd.AllRoutes {
		if num := len(strings.FieldsFunc(urlLocation, IsSlash)); num > maxURLFields {
			maxURLFields = num
		}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if global.EmergencyLockDown {
			/*
				An error response usually should carry status 5xx in this case, but the intention of
				emergency stop is to disable the program rather than crashing it and relaunching it.
				If an external trigger such as load balancer health check knocks on HTTP endpoint and relaunches
				the program after consecutive HTTP failures, it would defeat the intention of emergency stop.
				Hence the status code here is OK.
			*/
			w.Write([]byte(global.ErrEmergencyLockDown.Error()))
			return
		}
		urlFields := strings.FieldsFunc(r.URL.Path, IsSlash)
		// Retrieve part of requested URL that can be used to identify route
		assembledPath := "/"
		if len(urlFields) >= maxURLFields {
			assembledPath += strings.Join(urlFields[0:maxURLFields], "/")
		} else {
			assembledPath += strings.Join(urlFields, "/")
		}
		if pathLen := len(assembledPath); pathLen != 1 && assembledPath[pathLen-1] == '/' {
			assembledPath = assembledPath[0 : pathLen-1]
		}
		remoteIP := r.RemoteAddr[:strings.LastIndexByte(r.RemoteAddr, ':')]
		// Apply rate limit
		if limit, routeFound := httpd.AllRateLimits[assembledPath]; routeFound {
			if limit.Add(remoteIP, true) {
				// Look up the partial URL to find handler function
				httpd.Logger.Printf("Handle", remoteIP, nil, "%s %s", r.Method, assembledPath)
				httpd.AllRoutes[assembledPath](w, r)
			} else {
				http.Error(w, "", http.StatusTooManyRequests)
			}
		} else {
			// Route is not found
			if httpd.AllRateLimits[RateLimit404Key].Add(remoteIP, true) {
				httpd.Logger.Warningf("Handle", remoteIP, nil, "NotFound %s %s", r.Method, assembledPath)
				http.Error(w, "", http.StatusNotFound)
			} else {
				http.Error(w, "", http.StatusTooManyRequests)
			}
		}
	}
}

/*
You may call this function only after having called Initialise()!
Start HTTP daemon and block until this program exits.
*/
func (httpd *HTTPD) StartAndBlock() error {
	if httpd.TLSCertPath == "" {
		httpd.Logger.Printf("StartAndBlock", "", nil, "going to listen for HTTP connections")
		if err := httpd.Server.ListenAndServe(); err != nil {
			return fmt.Errorf("HTTPD.StartAndBlock: failed to listen on %s:%d - %v", httpd.ListenAddress, httpd.ListenPort, err)
		}
	} else {
		httpd.Logger.Printf("StartAndBlock", "", nil, "going to listen for HTTPS connections")
		if err := httpd.Server.ListenAndServeTLS(httpd.TLSCertPath, httpd.TLSKeyPath); err != nil {
			return fmt.Errorf("HTTPD.StartAndBlock: failed to listen on %s:%d - %v", httpd.ListenAddress, httpd.ListenPort, err)
		}
	}
	// Never reached
	return nil
}

// Run unit tests on API handlers of an already started HTTP daemon all API handlers. Essentially, it tests "api" package.
func TestAPIHandlers(httpd *HTTPD, t *testing.T) {
	// When accesses via HTTP, API handlers warn user about safety concern via a authorization prompt.
	basicAuth := map[string][]string{"Authorization": {"Basic Og=="}}
	addr := fmt.Sprintf("http://%s:%d", httpd.ListenAddress, httpd.ListenPort)
	// System information
	resp, err := httpclient.DoHTTP(httpclient.Request{Header: basicAuth}, addr+"/info")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "Stack traces:") {
		t.Fatal(err, string(resp.Body))
	}
	// Command Form
	resp, err = httpclient.DoHTTP(httpclient.Request{Header: basicAuth}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{Method: http.MethodPost, Header: basicAuth}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"cmd": {"verysecret.sls /"}}.Encode()),
	}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "bin") {
		t.Fatal(err, string(resp.Body))
	}
	// Gitlab handle
	resp, err = httpclient.DoHTTP(httpclient.Request{Header: basicAuth}, addr+"/gitlab")
	if err != nil || resp.StatusCode != http.StatusOK || strings.Index(string(resp.Body), "Enter path to browse") == -1 {
		t.Fatal(err, string(resp.Body), resp)
	}
	// HTML file
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/html")
	expected := "this is index 127.0.0.1 " + time.Now().Format(time.RFC3339)
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body), expected, resp)
	}
	// MailMe
	resp, err = httpclient.DoHTTP(httpclient.Request{Header: basicAuth}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{Method: http.MethodPost, Header: basicAuth}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"msg": {"又给你发了一个邮件"}}.Encode()),
	}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK ||
		(!strings.Contains(string(resp.Body), "发不出去") && !strings.Contains(string(resp.Body), "发出去了")) {
		t.Fatal(err, string(resp.Body))
	}
	// Proxy (visit https://github.com)
	resp, err = httpclient.DoHTTP(httpclient.Request{Header: basicAuth}, addr+"/proxy?u=https%%3A%%2F%%2Fgithub.com")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "github") || !strings.Contains(string(resp.Body), "laitos_rewrite_url") {
		t.Fatal(err, resp.StatusCode, string(resp.Body))
	}

	// Twilio - exchange SMS with bad PIN
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"Body": {"pin mismatch"}}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&api.HandleTwilioSMSHook{}))
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, resp)
	}
	// Twilio - exchange SMS, the extra spaces around prefix and PIN do not matter.
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"Body": {"verysecret .s echo 0123456789012345678901234567890123456789"}}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&api.HandleTwilioSMSHook{}))
	expected = `<?xml version="1.0" encoding="UTF-8"?>
<Response><Message><![CDATA[01234567890123456789012345678901234]]></Message></Response>
`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, resp)
	}
	// Twilio - check phone call greeting
	resp, err = httpclient.DoHTTP(httpclient.Request{Header: basicAuth}, addr+"/call_greeting")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say><![CDATA[Hi there]]></Say>`) {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - check phone call response to DTMF
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"Digits": {"0000000"}}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&api.HandleTwilioCallCallback{}))
	expected = `<?xml version="1.0" encoding="UTF-8"?>
<Response>
	<Say>Sorry</Say>
	<Hangup/>
</Response>
`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - check phone call response to command
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Header: basicAuth,
		//                                             v  e r  y  s   e c  r  e t .   s    tr  u e
		Body: strings.NewReader(url.Values{"Digits": {"88833777999777733222777338014207777087778833"}}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&api.HandleTwilioCallCallback{}))
	sayResp := `<Say><![CDATA[EMPTY OUTPUT, repeat again, EMPTY OUTPUT, repeat again, EMPTY OUTPUT, over.]]></Say>`
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), sayResp) {
		t.Fatal(err, string(resp.Body))
	}
}

// Start HTTP daemon and run unit tests on it. See TestHTTPD_StartAndBlock for daemon setup.
func TestHTTPD(httpd *HTTPD, t *testing.T) {
	// Create a temporary directory of file
	// Caller is supposed to set up the handler on /my/dir
	htmlDir := "/tmp/test-laitos-dir"
	if err := os.MkdirAll(htmlDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(htmlDir+"/a.html", []byte("a html"), 0644); err != nil {
		t.Fatal(err)
	}
	// HTTP daemon is expected to start in two seconds
	go func() {
		if err := httpd.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)

	addr := fmt.Sprintf("http://%s:%d", httpd.ListenAddress, httpd.ListenPort)

	// Directory handle
	resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != `<pre>
<a href="a.html">a.html</a>
</pre>
` {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir/a.html")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != "a html" {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Non-existent paths
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir/doesnotexist.html")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/doesnotexist")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Test hitting rate limits
	time.Sleep(RateLimitIntervalSec * time.Second)
	success := 0
	for i := 0; i < httpd.BaseRateLimit*DirectoryHandlerRateLimitFactor*2; i++ {
		resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir")
		if err == nil && resp.StatusCode == http.StatusOK {
			success++
		}
	}
	// Assume HTTPD's BaseRateLimit is 10
	if success > httpd.BaseRateLimit*DirectoryHandlerRateLimitFactor*3/2 ||
		success < httpd.BaseRateLimit*DirectoryHandlerRateLimitFactor/2 {
		t.Fatal(success)
	}
	// Wait till rate limits reset
	time.Sleep(RateLimitIntervalSec * time.Second)
	// Visit page again after rate limit resets
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != `<pre>
<a href="a.html">a.html</a>
</pre>
` {
		t.Fatal(err, string(resp.Body), resp)
	}
}
