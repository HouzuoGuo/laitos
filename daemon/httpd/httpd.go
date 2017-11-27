package httpd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/httpd/handler"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DirectoryHandlerRateLimitFactor = 3  // 2 times less expensive than the most expensive handler
	RateLimitIntervalSec            = 1  // Rate limit is calculated at 1 second interval
	IOTimeoutSec                    = 60 // IO timeout for both read and write operations
)

// HandlerCollection is a mapping between URL and implementation of handlers. It does not contain directory handlers.
type HandlerCollection map[string]handler.Handler

// SelfTest invokes self test function on all special handlers and report back if any error is encountered.
func (col HandlerCollection) SelfTest() error {
	ret := make([]string, 0, 0)
	retMutex := &sync.Mutex{}
	wait := &sync.WaitGroup{}
	wait.Add(len(col))
	for _, hand := range col {
		go func(handler handler.Handler) {
			err := handler.SelfTest()
			if err != nil {
				retMutex.Lock()
				ret = append(ret, err.Error())
				retMutex.Unlock()
			}
			wait.Done()
		}(hand)
	}
	wait.Wait()
	if len(ret) == 0 {
		return nil
	}
	return errors.New(strings.Join(ret, " | "))
}

// Generic HTTP daemon.
type Daemon struct {
	Address          string            `json:"Address"`          // Network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	Port             int               `json:"Port"`             // Port number to listen on
	TLSCertPath      string            `json:"TLSCertPath"`      // (Optional) serve HTTPS via this certificate
	TLSKeyPath       string            `json:"TLSKeyPath"`       // (Optional) serve HTTPS via this certificate (key)
	PerIPLimit       int               `json:"PerIPLimit"`       // PerIPLimit is approximately how many concurrent users are expected to be using the server from same IP address
	ServeDirectories map[string]string `json:"ServeDirectories"` // Serve directories (value) on prefix paths (key)

	HandlerCollection HandlerCollection          `json:"-"` // Specialised handlers that implement handler.HandlerFactory interface
	Processor         *common.CommandProcessor   `json:"-"` // Feature command processor
	AllRateLimits     map[string]*misc.RateLimit `json:"-"` // Aggregate all routes and their rate limit counters

	mux           *http.ServeMux
	serverWithTLS *http.Server // serverWithTLS is an instance of HTTP server that will be started with TLS listener.
	serverNoTLS   *http.Server // serverWithTLS is an instance of HTTP server that will be started with an ordinary listener.
	logger        misc.Logger
}

// Return path to Handler among special handlers that matches the specified type. Primarily used by test case code.
func (daemon *Daemon) GetHandlerByFactoryType(match handler.Handler) string {
	matchTypeString := reflect.TypeOf(match).String()
	for path, hand := range daemon.HandlerCollection {
		if reflect.TypeOf(hand).String() == matchTypeString {
			return path
		}
	}
	return ""
}

// RateLimitMiddleware acts against rate limited clients and global lockdown.
func (daemon *Daemon) Middleware(ratelimit *misc.RateLimit, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Put query duration (including IO time) into statistics
		beginTimeNano := time.Now().UnixNano()
		if misc.EmergencyLockDown {
			/*
				An error response usually should carry status 5xx in this case, but the intention of
				emergency stop is to disable the program rather than crashing it and relaunching it.
				If an external trigger such as load balancer health check knocks on HTTP endpoint and relaunches
				the program after consecutive HTTP failures, it would defeat the intention of emergency stop.
				Hence the status code here is OK.
			*/
			w.Write([]byte(misc.ErrEmergencyLockDown.Error()))
			handler.DurationStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
			return
		}
		// Check client IP against rate limit
		remoteIP := handler.GetRealClientIP(r)
		if ratelimit.Add(remoteIP, true) {
			daemon.logger.Printf("Handler", remoteIP, nil, "%s %s", r.Method, r.URL.Path)
			next(w, r)
		} else {
			http.Error(w, "", http.StatusTooManyRequests)
		}
		handler.DurationStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}
}

// Check configuration and initialise internal states.
func (daemon *Daemon) Initialise() error {
	daemon.logger = misc.Logger{ComponentName: "httpd", ComponentID: fmt.Sprintf("%s:%d", daemon.Address, daemon.Port)}
	if daemon.Processor == nil || daemon.Processor.IsEmpty() {
		daemon.logger.Printf("Initialise", "", nil, "daemon will not be able to execute toolbox commands due to lack of command processor filter configuration")
		daemon.Processor = common.GetEmptyCommandProcessor()
	}
	daemon.Processor.SetLogger(daemon.logger)
	if errs := daemon.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("httpd.Initialise: %+v", errs)
	}
	if daemon.Address == "" {
		return errors.New("httpd.Initialise: listen address is empty")
	}
	if daemon.Port < 1 {
		return errors.New("httpd.Initialise: listen port must be greater than 0")
	}
	if daemon.PerIPLimit < 1 {
		return errors.New("httpd.Initialise: PerIPLimit must be greater than 0")
	}
	if (daemon.TLSCertPath != "" || daemon.TLSKeyPath != "") && (daemon.TLSCertPath == "" || daemon.TLSKeyPath == "") {
		return errors.New("httpd.Initialise: missing TLS certificate or key path")
	}
	// Install handlers with rate-limiting middleware
	daemon.mux = new(http.ServeMux)
	daemon.AllRateLimits = map[string]*misc.RateLimit{}
	// Collect directory handlers
	if daemon.ServeDirectories != nil {
		for urlLocation, dirPath := range daemon.ServeDirectories {
			if urlLocation == "" || dirPath == "" {
				continue
			}
			if urlLocation[0] != '/' {
				urlLocation = "/" + urlLocation
			}
			if urlLocation[len(urlLocation)-1] != '/' {
				urlLocation += "/"
			}
			rl := &misc.RateLimit{
				UnitSecs: RateLimitIntervalSec,
				MaxCount: DirectoryHandlerRateLimitFactor * daemon.PerIPLimit,
				Logger:   daemon.logger,
			}
			daemon.AllRateLimits[urlLocation] = rl
			daemon.mux.HandleFunc(urlLocation, daemon.Middleware(rl, http.StripPrefix(urlLocation, http.FileServer(http.Dir(dirPath))).(http.HandlerFunc)))
		}
	}
	// Collect specialised handlers
	for urlLocation, hand := range daemon.HandlerCollection {
		if err := hand.Initialise(daemon.logger, daemon.Processor); err != nil {
			return err
		}
		rl := &misc.RateLimit{
			UnitSecs: RateLimitIntervalSec,
			MaxCount: hand.GetRateLimitFactor() * daemon.PerIPLimit,
			Logger:   daemon.logger,
		}
		daemon.AllRateLimits[urlLocation] = rl
		daemon.mux.HandleFunc(urlLocation, daemon.Middleware(rl, hand.Handle))
	}
	// Initialise all rate limits
	for _, limit := range daemon.AllRateLimits {
		limit.Initialise()
	}
	return nil
}

/*
StartAndBlockNoTLS starts HTTP daemon and serve unencrypted connections. Blocks caller until StopNoTLS function is called.
You may call this function only after having called Initialise()!
*/
func (daemon *Daemon) StartAndBlockNoTLS(fallbackPort int) error {
	/*
		In order to determine the listener's port:
		- On ElasticBeanstalk (and several other scenarios), listen on port number specified in environment PORT value.
		- If TLS configuration does not exist, listen on the port specified by user configuration.
		- If TLS configuration exists, then listen on the fallback port.
		Not very elegant, but it should help to launch HTTP daemon in TLS only, TLS + HTTP, and HTTP only scenarios.
	*/
	var port int
	if envPort := strings.TrimSpace(os.Getenv("PORT")); envPort == "" {
		if daemon.TLSCertPath == "" {
			port = daemon.Port
		} else {
			port = fallbackPort
		}
	} else {
		iPort, err := strconv.Atoi(envPort)
		if err != nil {
			return fmt.Errorf("httpd.StartAndBlockNoTLS: environment variable PORT value \"%s\" is not an integer", envPort)
		}
		port = iPort
	}
	// Configure servers with rather generous and sane defaults
	daemon.serverNoTLS = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", daemon.Address, port),
		Handler:      daemon.mux,
		ReadTimeout:  IOTimeoutSec * time.Second,
		WriteTimeout: IOTimeoutSec * time.Second,
	}
	daemon.logger.Printf("StartAndBlockNoTLS", "", nil, "going to listen for HTTP connections")
	if err := daemon.serverNoTLS.ListenAndServe(); err != nil {
		if strings.Contains(err.Error(), "closed") {
			return nil
		}
		return fmt.Errorf("httpd.StartAndBlockNoTLS: failed to listen on %s:%d - %v", daemon.Address, daemon.Port, err)
	}
	return nil
}

/*
StartAndBlockWithTLS starts HTTP daemon and serve encrypted connections. Blocks caller until StopNoTLS function is called.
You may call this function only after having called Initialise()!
*/
func (daemon *Daemon) StartAndBlockWithTLS() error {
	daemon.serverWithTLS = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", daemon.Address, daemon.Port),
		Handler:      daemon.mux,
		ReadTimeout:  IOTimeoutSec * time.Second,
		WriteTimeout: IOTimeoutSec * time.Second,
	}
	daemon.logger.Printf("StartAndBlockWithTLS", "", nil, "going to listen for HTTPS connections")
	if err := daemon.serverWithTLS.ListenAndServeTLS(daemon.TLSCertPath, daemon.TLSKeyPath); err != nil {
		if strings.Contains(err.Error(), "closed") {
			return nil
		}
		return fmt.Errorf("httpd.StartAndBlockWithTLS: failed to listen on %s:%d - %v", daemon.Address, daemon.Port, err)
	}
	return nil
}

// Stop HTTP daemon - the listener without TLS.
func (daemon *Daemon) StopNoTLS() {
	constraints, cancel := context.WithTimeout(context.Background(), time.Duration(IOTimeoutSec+2)*time.Second)
	defer cancel()
	if err := daemon.serverNoTLS.Shutdown(constraints); err != nil {
		daemon.logger.Warningf("StopNoTLS", "", err, "failed to shutdown")
	}
}

// Stop HTTP daemon - the listener with TLS.
func (daemon *Daemon) StopTLS() {
	constraints, cancel := context.WithTimeout(context.Background(), time.Duration(IOTimeoutSec+2)*time.Second)
	defer cancel()
	if err := daemon.serverWithTLS.Shutdown(constraints); err != nil {
		daemon.logger.Warningf("StopTLS", "", err, "failed to shutdown")
	}
}

// Run unit tests on API handlers of an already started HTTP daemon all API handlers. Essentially, it tests "handler" package.
func TestAPIHandlers(httpd *Daemon, t testingstub.T) {
	// When accesses via HTTP, API handlers warn user about safety concern via a authorization prompt.
	basicAuth := map[string][]string{"Authorization": {"Basic Og=="}}
	addr := fmt.Sprintf("http://%s:%d", httpd.Address, httpd.Port)
	// System information
	resp, err := inet.DoHTTP(inet.HTTPRequest{Header: basicAuth}, addr+"/info")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "Stack traces:") {
		t.Fatal(err, string(resp.Body))
	}
	// Command Form
	resp, err = inet.DoHTTP(inet.HTTPRequest{Header: basicAuth}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = inet.DoHTTP(inet.HTTPRequest{Method: http.MethodPost, Header: basicAuth}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"cmd": {"verysecret.sls /"}}.Encode()),
	}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "bin") {
		t.Fatal(err, string(resp.Body))
	}
	// Gitlab handle
	resp, err = inet.DoHTTP(inet.HTTPRequest{Header: basicAuth}, addr+"/gitlab")
	if err != nil || resp.StatusCode != http.StatusOK || strings.Index(string(resp.Body), "Enter path to browse") == -1 {
		t.Fatal(err, string(resp.Body), resp)
	}
	// HTML file
	resp, err = inet.DoHTTP(inet.HTTPRequest{}, addr+"/html")
	expected := "this is index 127.0.0.1 " + time.Now().Format(time.RFC3339)
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body), expected, resp)
	}
	// MailMe
	resp, err = inet.DoHTTP(inet.HTTPRequest{Header: basicAuth}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = inet.DoHTTP(inet.HTTPRequest{Method: http.MethodPost, Header: basicAuth}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"msg": {"又给你发了一个邮件"}}.Encode()),
	}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK ||
		(!strings.Contains(string(resp.Body), "发不出去") && !strings.Contains(string(resp.Body), "发出去了")) {
		t.Fatal(err, string(resp.Body))
	}
	// Microsoft bot
	resp, err = inet.DoHTTP(inet.HTTPRequest{Header: basicAuth}, addr+"/microsoft_bot")
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatal(err, string(resp.Body))
	}
	microsoftBotDummyChat := handler.MicrosoftBotIncomingChat{}
	var microsoftBotDummyChatRequest []byte
	if microsoftBotDummyChatRequest, err = json.Marshal(microsoftBotDummyChat); err != nil {
		t.Fatal(err)
	}
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Header: basicAuth,
		Body:   bytes.NewReader(microsoftBotDummyChatRequest)}, addr+"/microsoft_bot")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatal(err, string(resp.Body))
	}
	// Proxy (visit https://github.com)
	resp, err = inet.DoHTTP(inet.HTTPRequest{Header: basicAuth}, addr+"/proxy?u=https%%3A%%2F%%2Fgithub.com")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "github") || !strings.Contains(string(resp.Body), "laitos_rewrite_url") {
		t.Fatal(err, resp.StatusCode, string(resp.Body))
	}

	// Twilio - exchange SMS with bad PIN
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"Body": {"incorrect PIN"}}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioSMSHook{}))
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Message><![CDATA[Failed to match PIN/shortcut]]></Message>`) {
		t.Fatal(err, resp)
	}
	// Twilio - exchange SMS, the extra spaces around prefix and PIN do not matter.
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"Body": {"verysecret .s echo 0123456789012345678901234567890123456789"}}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioSMSHook{}))
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<![CDATA[01234567890123456789012345678901234]]>`) {
		t.Fatal(err, resp)
	}
	// Twilio - prevent SMS spam according to incoming phone number
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body: strings.NewReader(url.Values{
			"Body": {"verysecret .s echo 0123456789012345678901234567890123456789"},
			"From": {"sms number"},
		}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioSMSHook{}))
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<![CDATA[01234567890123456789012345678901234]]>`) {
		t.Fatal(err, resp)
	}
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body: strings.NewReader(url.Values{
			"Body": {"verysecret .s echo 0123456789012345678901234567890123456789"},
			"From": {"sms number"},
		}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioSMSHook{}))
	if err != nil || resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(resp.Body), `rate limit is exceeded by`) {
		t.Fatal(err, resp)
	}
	// Twilio - check phone call greeting
	resp, err = inet.DoHTTP(inet.HTTPRequest{Header: basicAuth}, addr+"/call_greeting")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say><![CDATA[Hi there]]></Say>`) {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - prevent call spam according to incoming phone number
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"From": {"call number"}}.Encode()),
	}, addr+"/call_greeting")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say><![CDATA[Hi there]]></Say>`) {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"From": {"call number"}}.Encode()),
	}, addr+"/call_greeting")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Response><Reject/></Response>`) {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - check phone call response to DTMF
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"Digits": {"0000000"}}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioCallCallback{}))
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `Failed to match PIN/shortcut`) {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - check command execution result via phone call
	//                         v  e r  y  s   e c  r  e t .   s    tr  u e
	dtmfVerySecretDotSTrue := "88833777999777733222777338014207777087778833"
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"Digits": {dtmfVerySecretDotSTrue}}.Encode())}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioCallCallback{}))
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say><![CDATA[EMPTY OUTPUT, repeat again, EMPTY OUTPUT, repeat again, EMPTY OUTPUT, over.]]></Say>`) {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - check command execution result via phone call and ask output to be spelt phonetically
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"Digits": {handler.TwilioPhoneticSpellingMagic + dtmfVerySecretDotSTrue}}.Encode())}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioCallCallback{}))
	phoneticOutput := `capital echo, capital mike, capital papa, capital tango, capital yankee, space, capital oscar, capital uniform, capital tango, capital papa, capital uniform, capital tango, repeat again, capital echo, capital mike, capital papa, capital tango, capital yankee, space, capital oscar, capital uniform, capital tango, capital papa, capital uniform, capital tango, repeat again, capital echo, capital mike, capital papa, capital tango, capital yankee, space, capital oscar, capital uniform, capital tango, capital papa, capital uniform, capital tango, over.`
	sayResp := `<Say><![CDATA[` + phoneticOutput + `]]></Say>`
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), sayResp) {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - prevent DTMF command spam according to incoming phone number
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"Digits": {dtmfVerySecretDotSTrue}, "From": {"dtmf number"}}.Encode())}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioCallCallback{}))
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say><![CDATA[EMPTY OUTPUT, repeat again, EMPTY OUTPUT, repeat again, EMPTY OUTPUT, over.]]></Say>`) {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"Digits": {dtmfVerySecretDotSTrue}, "From": {"dtmf number"}}.Encode())}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioCallCallback{}))
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say>You are rate limited.</Say><Hangup/>`) {
		t.Fatal(err, string(resp.Body))
	}
	// Wait for phone number rate limit to expire for SMS, call, and DTMF command, then redo the tests
	time.Sleep((handler.TwilioPhoneNumberRateLimitIntervalSec + 1) * time.Second)
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body: strings.NewReader(url.Values{
			"Body": {"verysecret .s echo 0123456789012345678901234567890123456789"},
			"From": {"sms number"},
		}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioSMSHook{}))
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<![CDATA[01234567890123456789012345678901234]]>`) {
		t.Fatal(err, resp)
	}
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"From": {"call number"}}.Encode()),
	}, addr+"/call_greeting")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say><![CDATA[Hi there]]></Say>`) {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = inet.DoHTTP(inet.HTTPRequest{
		Method: http.MethodPost,
		Header: basicAuth,
		Body:   strings.NewReader(url.Values{"Digits": {dtmfVerySecretDotSTrue}, "From": {"dtmf number"}}.Encode())}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioCallCallback{}))
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say><![CDATA[EMPTY OUTPUT, repeat again, EMPTY OUTPUT, repeat again, EMPTY OUTPUT, over.]]></Say>`) {
		t.Fatal(err, string(resp.Body))
	}
}

// Run unit test on HTTP daemon. See TestHTTPD_StartAndBlock for daemon setup.
func TestHTTPD(httpd *Daemon, t testingstub.T) {
	// Create a temporary directory of file
	// Caller is supposed to set up the handler on /my/dir
	htmlDir := "/tmp/test-laitos-dir"
	if err := os.MkdirAll(htmlDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(htmlDir+"/a.html", []byte("a html"), 0644); err != nil {
		t.Fatal(err)
	}

	addr := fmt.Sprintf("http://%s:%d", httpd.Address, httpd.Port)

	// Directory handle
	resp, err := inet.DoHTTP(inet.HTTPRequest{}, addr+"/my/dir")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != `<pre>
<a href="a.html">a.html</a>
</pre>
` {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = inet.DoHTTP(inet.HTTPRequest{}, addr+"/my/dir/a.html")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != "a html" {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = inet.DoHTTP(inet.HTTPRequest{}, addr+"/dir")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != `<pre>
<a href="a.html">a.html</a>
</pre>
` {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = inet.DoHTTP(inet.HTTPRequest{}, addr+"/dir/a.html")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != "a html" {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Non-existent path in directory
	resp, err = inet.DoHTTP(inet.HTTPRequest{}, addr+"/my/dir/doesnotexist.html")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = inet.DoHTTP(inet.HTTPRequest{}, addr+"/doesnotexist")
	// Non-existent path, but go is quite stupid that it produces response of /
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Test hitting rate limits
	time.Sleep(RateLimitIntervalSec * time.Second)
	success := 0
	for i := 0; i < httpd.PerIPLimit*DirectoryHandlerRateLimitFactor*2; i++ {
		resp, err = inet.DoHTTP(inet.HTTPRequest{}, addr+"/my/dir/a.html")
		if err == nil && resp.StatusCode == http.StatusOK {
			success++
		}
	}
	if success < 1 || success > httpd.PerIPLimit*DirectoryHandlerRateLimitFactor*2 {
		t.Fatal(success)
	}
	// Wait out rate limit (leave 3 seconds buffer for pending requests to complete)
	time.Sleep((RateLimitIntervalSec + 3) * time.Second)
	// Visit page again after rate limit resets
	resp, err = inet.DoHTTP(inet.HTTPRequest{}, addr+"/my/dir")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != `<pre>
<a href="a.html">a.html</a>
</pre>
` {
		t.Fatal(err, string(resp.Body), resp)
	}
}
