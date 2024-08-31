package httpd

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/httpd/handler"
	"github.com/HouzuoGuo/laitos/daemon/httpd/middleware"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

const (
	DirectoryHandlerRateLimitFactor = 8  // DirectoryHandlerRateLimitFactor is 7 times less expensive than the most expensive handler
	RateLimitIntervalSec            = 1  // Rate limit is calculated at 1 second interval
	IOTimeoutSec                    = 60 // IO timeout for both read and write operations

	// MaxRequestBodyBytes is the maximum size (in bytes) of a request body that HTTP server will process for a request.
	MaxRequestBodyBytes = 1024 * 1024

	// EnvironmentPortNumber is the name of an environment variable, the value
	// of which determines the port number HTTP (no TLS) server will listen on.
	// As a special consideration, the variable name "PORT" is conventionally
	// used by a few public cloud services (e.g. AWS Elastic Beanstalk) to
	// inform an application which port it should listen on.
	EnvironmentPortNumber = "PORT"

	// EnvironmentIndexPage is the name of an environment variable that provides
	// an way of installing an HTML doc handler for an index page instead of
	// writing the same configuration in JSON configuration.
	// When the environment variable is present, the HTTP daemons will
	// initialise themselves with an HTML doc handler serving the environment
	// variable content at "/", "/index.htm", and "/index.html".
	// This environment variable value takes precedence over JSON configuration.
	EnvironmentIndexPage = "LAITOS_INDEX_PAGE"
)

// HandlerCollection is a mapping between URL and implementation of handlers. It does not contain directory handlers.
type HandlerCollection map[string]handler.Handler

// SelfTest invokes self test function on all special handlers and report back if any error is encountered.
func (col HandlerCollection) SelfTest() error {
	ret := make([]string, 0)
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
	PlainPort        int               `json:"-"`                // PlainPort is assigned to the port used by NoTLS listener once it starts.
	TLSCertPath      string            `json:"TLSCertPath"`      // (Optional) serve HTTPS via this certificate
	TLSKeyPath       string            `json:"TLSKeyPath"`       // (Optional) serve HTTPS via this certificate (key)
	PerIPLimit       int               `json:"PerIPLimit"`       // PerIPLimit is approximately how many concurrent users are expected to be using the server from same IP address
	ServeDirectories map[string]string `json:"ServeDirectories"` // Serve directories (value) on prefix paths (key)

	HandlerCollection HandlerCollection         `json:"-"` // Specialised handlers that implement handler.HandlerFactory interface
	Processor         *toolbox.CommandProcessor `json:"-"` // Feature command processor
	// ResourcePaths is the whole collection of URLs handled by the server.
	ResourcePaths map[string]struct{} `json:"-"`

	mux           *http.ServeMux
	serverWithTLS *http.Server // serverWithTLS is an instance of HTTP server that will be started with TLS listener.
	serverNoTLS   *http.Server // serverWithTLS is an instance of HTTP server that will be started with an ordinary listener.
	logger        *lalog.Logger
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

/*
Initialise validates daemon configuration and initialises internal states.

stripURLPrefixFromRequest is an optional prefix string that is expected to be present in request URLs.
If used, HTTP server will install all of its handlers at URL location according to the server configuration, but with the prefix
URL string added to each of them.
This often helps when some kind of API gateway (e.g. AWS API gateway) proxies visitors' requests and places a prefix string in
each request.
For example: a homepage's domain is served by a CDN, the CDN forwards visitors' requests to a backend ("origin") and in doing
so automatically adds a URL prefix "/stageLive" because the backend expects such prefix. In this case, the stripURLPrefixFromRequest
shall be "/stageLive".

stripURLPrefixFromResponse is an optional prefix string that will be stirpped from rendered HTML pages, such as links on pages and
form action URLs, this is usually used in conjunction with stripURLPrefixFromRequest.
*/
func (daemon *Daemon) Initialise(stripURLPrefixFromRequest string, stripURLPrefixFromResponse string) error {
	if daemon.Address == "" {
		daemon.Address = "0.0.0.0"
	}
	if daemon.Port < 1 {
		if daemon.TLSCertPath == "" {
			daemon.Port = 80
		} else {
			daemon.Port = 443
		}
	}
	if daemon.PerIPLimit < 1 {
		daemon.PerIPLimit = 12 // reasonable for couple of users that use advanced API endpoints in parallel
	}
	daemon.logger = &lalog.Logger{
		ComponentName: "httpd",
		ComponentID:   []lalog.LoggerIDField{{Key: "Port", Value: daemon.Port}},
	}
	if daemon.Processor == nil || daemon.Processor.IsEmpty() {
		daemon.logger.Info("", nil, "daemon will not be able to execute toolbox commands due to lack of command processor filter configuration")
		daemon.Processor = toolbox.GetEmptyCommandProcessor()
	}
	daemon.Processor.SetLogger(daemon.logger)
	if errs := daemon.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("httpd.Initialise: %+v", errs)
	}
	if (daemon.TLSCertPath != "" || daemon.TLSKeyPath != "") && (daemon.TLSCertPath == "" || daemon.TLSKeyPath == "") {
		return errors.New("httpd.Initialise: missing TLS certificate or key path")
	}

	// Install handlers with rate-limiting middleware
	daemon.mux = new(http.ServeMux)
	if daemon.HandlerCollection == nil {
		daemon.HandlerCollection = HandlerCollection{}
	}
	daemon.ResourcePaths = make(map[string]struct{})

	// Prometheus histograms that use a label to tell the HTTP handler associated with the histogram metrics
	var handlerDurationHistogram, responseTimeToFirstByteHistogram, responseSizeHistogram *prometheus.HistogramVec
	if misc.EnablePrometheusIntegration {
		metricsLabelNames := []string{middleware.PrometheusHandlerTypeLabel, middleware.PrometheusHandlerLocationLabel}
		handlerDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "laitos_httpd_handler_duration_seconds",
			Help:    "The run-duration of HTTP handler function in seconds",
			Buckets: []float64{0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
		}, metricsLabelNames)
		responseTimeToFirstByteHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "laitos_httpd_response_time_to_first_byte_seconds",
			Help:    "The time-to-first-byte of HTTP handler function in seconds",
			Buckets: []float64{0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
		}, metricsLabelNames)
		responseSizeHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "laitos_httpd_response_size_bytes",
			Help:    "The size of response produced by HTTP handler function in bytes",
			Buckets: []float64{256, 1024, 4096, 16384, 65536, 262144, 1048576},
		}, metricsLabelNames)
		for _, histogram := range []*prometheus.HistogramVec{handlerDurationHistogram, responseTimeToFirstByteHistogram, responseSizeHistogram} {
			if err := prometheus.Register(histogram); err != nil {
				daemon.logger.Warning("", err, "failed to register prometheus metrics collectors")
			}
		}
	}

	// Install directory handlers.
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
			urlLocation = stripURLPrefixFromRequest + urlLocation
			rl := lalog.NewRateLimit(RateLimitIntervalSec, DirectoryHandlerRateLimitFactor*daemon.PerIPLimit, daemon.logger)
			daemon.ResourcePaths[urlLocation] = struct{}{}
			decoratedHandlerFunc := middleware.LogRequestStats(daemon.logger,
				middleware.RecordInternalStats(misc.HTTPDStats,
					middleware.EmergencyLockdown(
						middleware.RecordLatestRequests(daemon.logger,
							middleware.RecordPrometheusStats("FileServer", urlLocation, handlerDurationHistogram, responseTimeToFirstByteHistogram, responseSizeHistogram,
								middleware.RateLimit(rl,
									middleware.RestrictMaxRequestSize(MaxRequestBodyBytes,
										http.StripPrefix(urlLocation, http.FileServer(http.Dir(dirPath))).(http.HandlerFunc))))))))
			daemon.mux.Handle(urlLocation, decoratedHandlerFunc)
			daemon.logger.Info("", nil, "installed directory listing handler at location \"%s\"", urlLocation)
		}
	}

	// Install index page handlers.
	if indexPageContent := strings.TrimSpace(os.Getenv(EnvironmentIndexPage)); indexPageContent != "" {
		daemon.logger.Info("", nil, "serving index page from environment variable %s", EnvironmentIndexPage)
		indexHandler := &handler.HandleHTMLDocument{HTMLContent: indexPageContent}
		for _, path := range []string{"/", "/index.htm", "/index.html"} {
			daemon.HandlerCollection[path] = indexHandler
		}
	}

	// Install web service handlers.
	for urlLocation, hand := range daemon.HandlerCollection {
		if err := hand.Initialise(daemon.logger, daemon.Processor, stripURLPrefixFromResponse); err != nil {
			return err
		}
		rl := lalog.NewRateLimit(RateLimitIntervalSec, hand.GetRateLimitFactor()*daemon.PerIPLimit, daemon.logger)
		urlLocation = stripURLPrefixFromRequest + urlLocation
		daemon.ResourcePaths[urlLocation] = struct{}{}
		// With the exception of file upload handler, all handlers will be subject to a limited request size.
		_, unrestrictedRequestSize := hand.(*handler.HandleFileUpload)
		handlerTypeName := reflect.TypeOf(hand).String()
		innerMostHandler := hand.Handle
		if !unrestrictedRequestSize {
			innerMostHandler = middleware.RestrictMaxRequestSize(MaxRequestBodyBytes, innerMostHandler)
		}
		decoratedHandlerFunc := middleware.LogRequestStats(daemon.logger,
			middleware.RecordInternalStats(misc.HTTPDStats,
				middleware.EmergencyLockdown(
					middleware.RecordLatestRequests(daemon.logger,
						middleware.RecordPrometheusStats(handlerTypeName, urlLocation, handlerDurationHistogram, responseTimeToFirstByteHistogram, responseSizeHistogram,
							middleware.WithAWSXray(
								middleware.RateLimit(rl, innerMostHandler)))))))
		daemon.mux.Handle(urlLocation, decoratedHandlerFunc)
		daemon.logger.Info("", nil, "installed web service \"%s\" at location \"%s\"", handlerTypeName, urlLocation)
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
	if envPort := strings.TrimSpace(os.Getenv(EnvironmentPortNumber)); envPort == "" {
		if daemon.TLSCertPath == "" {
			daemon.PlainPort = daemon.Port
		} else {
			daemon.PlainPort = fallbackPort
		}
	} else {
		iPort, err := strconv.Atoi(envPort)
		if err != nil {
			return fmt.Errorf("httpd.StartAndBlockNoTLS: environment variable %s value \"%s\" is must be an integer", EnvironmentPortNumber, envPort)
		}
		daemon.PlainPort = iPort
	}
	// Configure servers with rather generous and sane defaults
	daemon.serverNoTLS = &http.Server{
		Addr:         net.JoinHostPort(daemon.Address, strconv.Itoa(daemon.PlainPort)),
		Handler:      daemon.mux,
		ReadTimeout:  IOTimeoutSec * time.Second,
		WriteTimeout: IOTimeoutSec * time.Second,
	}
	daemon.logger.Info("", nil, "going to listen for HTTP connections on port %d", daemon.PlainPort)
	if err := daemon.serverNoTLS.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("httpd.StartAndBlockNoTLS: failed to listen on %s:%d - %v", daemon.Address, daemon.Port, err)
	}
	return nil
}

/*
StartAndBlockWithTLS starts HTTP daemon and serve encrypted connections. Blocks caller until StopTLS function is called.
You may call this function only after having called Initialise()!
*/
func (daemon *Daemon) StartAndBlockWithTLS() error {
	contents, _, err := misc.DecryptIfNecessary(misc.ProgramDataDecryptionPassword, daemon.TLSCertPath, daemon.TLSKeyPath)
	if err != nil {
		return err
	}
	tlsCert, err := tls.X509KeyPair(contents[0], contents[1])
	if err != nil {
		return fmt.Errorf("httpd.StartAndBlockWithTLS: failed to load certificate or key - %v", err)
	}
	daemon.serverWithTLS = &http.Server{
		Addr:         net.JoinHostPort(daemon.Address, strconv.Itoa(daemon.Port)),
		Handler:      daemon.mux,
		ReadTimeout:  IOTimeoutSec * time.Second,
		WriteTimeout: IOTimeoutSec * time.Second,
		TLSConfig:    &tls.Config{Certificates: []tls.Certificate{tlsCert}},
	}
	daemon.logger.Info("", nil, "going to listen for HTTPS connections on port %d", daemon.Port)

	if err := daemon.serverWithTLS.ListenAndServeTLS("", ""); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("httpd.StartAndBlockWithTLS: failed to listen on %s:%d - %v", daemon.Address, daemon.Port, err)
	}
	return nil
}

// Stop HTTP daemon - the listener without TLS.
func (daemon *Daemon) StopNoTLS() {
	if server := daemon.serverNoTLS; server != nil {
		constraints, cancel := context.WithTimeout(context.Background(), time.Duration(IOTimeoutSec+2)*time.Second)
		defer cancel()
		if err := server.Shutdown(constraints); err != nil {
			daemon.logger.Warning(daemon.Address, err, "failed to shutdown")
		}
	}
}

// Stop HTTP daemon - the listener with TLS.
func (daemon *Daemon) StopTLS() {
	if server := daemon.serverWithTLS; server != nil {
		constraints, cancel := context.WithTimeout(context.Background(), time.Duration(IOTimeoutSec+2)*time.Second)
		defer cancel()
		if err := server.Shutdown(constraints); err != nil {
			daemon.logger.Warning(daemon.Address, err, "failed to shutdown")
		}
	}
}

// Run unit tests on API handlers of an already started HTTP daemon all API handlers. Essentially, it tests "handler" package.
func TestAPIHandlers(httpd *Daemon, t testingstub.T) {
	addr := fmt.Sprintf("http://%s:%d", httpd.Address, httpd.Port)
	// System information
	resp, err := inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/info")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "Stack traces:") {
		t.Fatal(err, string(resp.Body))
	}
	// Command Form
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{Method: http.MethodPost}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"cmd": {"verysecret.secho cmd_form_test"}}.Encode()),
	}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "cmd_form_test") {
		t.Fatal(err, string(resp.Body))
	}
	// File upload - home page
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/upload")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	// File upload - upload
	fileUploadRequestBody := &bytes.Buffer{}
	fileUploadRequestWriter := multipart.NewWriter(fileUploadRequestBody)
	fileUploadPart, err := fileUploadRequestWriter.CreateFormFile("upload", "sample-upload.html")
	if err != nil {
		t.Fatal(err)
	}
	uploadSourceFile, err := os.Open("/tmp/test-laitos-index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer uploadSourceFile.Close()
	if _, err = io.Copy(fileUploadPart, uploadSourceFile); err != nil {
		t.Fatal(err)
	}
	if err = fileUploadRequestWriter.WriteField("submit", "Upload"); err != nil {
		t.Fatal(err)
	}
	if err = fileUploadRequestWriter.Close(); err != nil {
		t.Fatal(err)
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method:      http.MethodPost,
		ContentType: fileUploadRequestWriter.FormDataContentType(),
		Body:        fileUploadRequestBody,
	}, addr+"/upload")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// File upload - download
	var downloadFileName string
	for _, line := range strings.Split(string(resp.Body), "\n") {
		/*
			The line looks like:
			<pre>.... Your file is available for 24 hours under name: xxxx.nnn</pre>
		*/
		if strings.Contains(line, "Your file is available for 24 hours") {
			downloadFileName = line[strings.IndexRune(line, ':')+1 : strings.LastIndexByte(line, '<')]
		}
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method:      http.MethodPost,
		ContentType: "application/x-www-form-urlencoded",
		Body:        strings.NewReader(url.Values{"submit": []string{"Download"}, "download": []string{downloadFileName}}.Encode()),
	}, addr+"/upload")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != TestLaitosIndexHTMLContent {
		t.Fatal(err, resp, string(resp.Body))
	}

	// Gitlab handle
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/gitlab")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "Enter path to browse") {
		t.Fatal(err, string(resp.Body), resp)
	}
	// HTML file
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/html")
	expected := "this is index 127.0.0.1 " + time.Now().Format(time.RFC3339)
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body), expected, resp)
	}
	// MailMe
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{Method: http.MethodPost}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"msg": {"又给你发了一个邮件"}}.Encode()),
	}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK ||
		(!strings.Contains(string(resp.Body), "发不出去") && !strings.Contains(string(resp.Body), "发出去了")) {
		t.Fatal(err, string(resp.Body))
	}
	// Microsoft bot
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/microsoft_bot")
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatal(err, string(resp.Body))
	}
	microsoftBotDummyChat := handler.MicrosoftBotIncomingChat{}
	var microsoftBotDummyChatRequest []byte
	if microsoftBotDummyChatRequest, err = json.Marshal(microsoftBotDummyChat); err != nil {
		t.Fatal(err)
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Body: bytes.NewReader(microsoftBotDummyChatRequest)}, addr+"/microsoft_bot")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatal(err, string(resp.Body))
	}
	// Recurring commands - setup page
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/recurring_cmds")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	// Recurring commands - retrieve results
	time.Sleep(time.Duration(httpd.HandlerCollection["/recurring_cmds"].(*handler.HandleRecurringCommands).RecurringCommands["channel2"].IntervalSec+1) * time.Second) // give timer commands a moment to trigger
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/recurring_cmds?retrieve=channel2")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "channel2") {
		t.Fatal(err, string(resp.Body))
	}
	// Proxy (visit https://github.com)
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/proxy?u=https%%3A%%2F%%2Fgithub.com")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "github") || !strings.Contains(string(resp.Body), "laitos_rewrite_url") {
		t.Fatal(err, resp.StatusCode, string(resp.Body))
	}
	// Twilio - exchange SMS with bad PIN
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"Body": {"incorrect PIN"}}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioSMSHook{}))
	if err != nil || resp.StatusCode != http.StatusServiceUnavailable || strings.TrimSpace(string(resp.Body)) != toolbox.ErrPINAndShortcutNotFound.Error() {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - exchange SMS, the extra spaces around prefix and PIN do not matter.
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"Body": {"verysecret .s echo 0123456789012345678901234567890123456789"}}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioSMSHook{}))
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `01234567890123456789012345678901234`) {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - prevent SMS spam according to incoming phone number
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body: strings.NewReader(url.Values{
			"Body": {"verysecret .s echo 0123456789012345678901234567890123456789"},
			"From": {"sms number"},
		}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioSMSHook{}))
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `01234567890123456789012345678901234`) {
		t.Fatal(err, resp)
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body: strings.NewReader(url.Values{
			"Body": {"verysecret .s echo 0123456789012345678901234567890123456789"},
			"From": {"sms number"},
		}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioSMSHook{}))
	if err != nil || resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(resp.Body), `rate limit is exceeded by`) {
		t.Fatalf("err? %v\nresp: %+v\nresp body: %s", err, resp, string(resp.Body))
	}
	// Twilio - check phone call greeting
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/call_greeting")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say>Hi there</Say>`) {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - prevent call spam according to incoming phone number
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"From": {"call number"}}.Encode()),
	}, addr+"/call_greeting")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say>Hi there</Say>`) {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"From": {"call number"}}.Encode()),
	}, addr+"/call_greeting")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Response><Reject/></Response>`) {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - check phone call response to DTMF
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"Digits": {"0000000"}}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioCallCallback{}))
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), toolbox.ErrPINAndShortcutNotFound.Error()) {
		t.Fatal(err, string(resp.Body))
	}
	//                         v  e r  y  s   e c  r  e t .   s    tr  u e
	dtmfVerySecretDotSTrue := "88833777999777733222777338014207777087778833"
	if !platform.HostIsWindows() {
		// Twilio - check command execution result via phone call
		resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
			Method: http.MethodPost,
			Body:   strings.NewReader(url.Values{"Digits": {dtmfVerySecretDotSTrue}}.Encode())}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioCallCallback{}))
		if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say>EMPTY OUTPUT.

    repeat again.    

EMPTY OUTPUT.

    repeat again.    

EMPTY OUTPUT.
over.
</Say>`) {
			t.Fatal(err, string(resp.Body))
		}
		// Twilio - check command execution result via phone call and ask output to be spelt phonetically
		resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
			Method: http.MethodPost,
			Body:   strings.NewReader(url.Values{"Digits": {handler.TwilioPhoneticSpellingMagic + dtmfVerySecretDotSTrue}}.Encode())}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioCallCallback{}))
		phoneticOutput := `capital echo, capital mike, capital papa, capital tango, capital yankee, space, capital oscar, capital uniform, capital tango, capital papa, capital uniform, capital tango.

    repeat again.    

capital echo, capital mike, capital papa, capital tango, capital yankee, space, capital oscar, capital uniform, capital tango, capital papa, capital uniform, capital tango.

    repeat again.    

capital echo, capital mike, capital papa, capital tango, capital yankee, space, capital oscar, capital uniform, capital tango, capital papa, capital uniform, capital tango.
over.`
		sayResp := "<Say>" + phoneticOutput + "\n</Say>"
		if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), sayResp) {
			t.Fatal(err, string(resp.Body))
		}
		// Twilio - prevent DTMF command spam according to incoming phone number
		resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
			Method: http.MethodPost,
			Body:   strings.NewReader(url.Values{"Digits": {dtmfVerySecretDotSTrue}, "From": {"dtmf number"}}.Encode())}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioCallCallback{}))
		if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say>EMPTY OUTPUT.

    repeat again.    

EMPTY OUTPUT.

    repeat again.    

EMPTY OUTPUT.
over.
</Say>`) {
			t.Fatal(err, string(resp.Body))
		}
		resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
			Method: http.MethodPost,
			Body:   strings.NewReader(url.Values{"Digits": {dtmfVerySecretDotSTrue}, "From": {"dtmf number"}}.Encode())}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioCallCallback{}))
		if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say>You are rate limited.</Say><Hangup/>`) {
			t.Fatal(err, string(resp.Body))
		}
	}

	// Wait for phone number rate limit to expire for SMS, call, and DTMF command, then redo the tests
	time.Sleep((handler.TwilioPhoneNumberRateLimitIntervalSec + 1) * time.Second)
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body: strings.NewReader(url.Values{
			"Body": {"verysecret .s echo 0123456789012345678901234567890123456789"},
			"From": {"sms number"},
		}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioSMSHook{}))
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `01234567890123456789012345678901234`) {
		t.Fatal(err, resp)
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"From": {"call number"}}.Encode()),
	}, addr+"/call_greeting")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say>Hi there</Say>`) {
		t.Fatal(err, string(resp.Body))
	}
	if !platform.HostIsWindows() {
		resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
			Method: http.MethodPost,
			Body:   strings.NewReader(url.Values{"Digits": {dtmfVerySecretDotSTrue}, "From": {"dtmf number"}}.Encode())}, addr+httpd.GetHandlerByFactoryType(&handler.HandleTwilioCallCallback{}))
		if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), `<Say>EMPTY OUTPUT.

    repeat again.    

EMPTY OUTPUT.

    repeat again.    

EMPTY OUTPUT.
over.
</Say>`) {
			t.Fatal(err, string(resp.Body))
		}
	}

	// Test app command execution endpoint
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"cmd": {toolbox.TestCommandProcessorPIN + ".s echo hi"}}.Encode())}, addr+httpd.GetHandlerByFactoryType(&handler.HandleAppCommand{}))
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != "hi" {
		t.Fatal(err, string(resp.Body))
	}

	// Test reports endpoint
	httpd.Processor.Features.MessageProcessor.StoreReport(context.Background(), toolbox.SubjectReportRequest{
		SubjectHostName: "subject-host-name",
	}, "client-ip1", "client-daemon1")
	httpd.Processor.Features.MessageProcessor.StoreReport(context.Background(), toolbox.SubjectReportRequest{
		SubjectHostName: "subject-host-name",
	}, "client-ip2", "client-daemon2")
	// Retrieve subjects and count of their reports - two reports from the same host, and one report coming from TTN.
	var subjectCount map[string]int
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{Method: http.MethodPost}, addr+httpd.GetHandlerByFactoryType(&handler.HandleReportsRetrieval{}))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatal(err, string(resp.Body))
	}
	if err := json.Unmarshal(resp.Body, &subjectCount); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(subjectCount, map[string]int{"subject-host-name": 2}) {
		t.Fatalf("%+v", subjectCount)
	}
	// Retrieve TTN report + two host reports from the latest to oldest
	var reports []toolbox.SubjectReport
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{Method: http.MethodPost}, addr+httpd.GetHandlerByFactoryType(&handler.HandleReportsRetrieval{})+"?n=100")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatal(err, string(resp.Body))
	}
	if err := json.Unmarshal(resp.Body, &reports); err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 || reports[0].SubjectClientTag != "client-ip2" || reports[1].SubjectClientTag != "client-ip1" {
		t.Fatalf("%+v", reports)
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"n": {"1"}, "host": {"subject-host-name"}}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&handler.HandleReportsRetrieval{}))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatal(err, string(resp.Body))
	}
	if err := json.Unmarshal(resp.Body, &reports); err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || reports[0].SubjectClientTag != "client-ip2" {
		t.Fatalf("%+v", reports)
	}
	// Assign subject a command to run
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"host": {"subject-host-name"}, "cmd": {"test123"}}.Encode()),
	}, addr+httpd.GetHandlerByFactoryType(&handler.HandleReportsRetrieval{}))
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "will carry an app command") {
		t.Fatal(err, string(resp.Body))
	}
	if cmd := httpd.Processor.Features.MessageProcessor.OutgoingAppCommands["subject-host-name"]; cmd != "test123" {
		t.Fatal(cmd)
	}
}

const (
	TestLaitosIndexHTMLContent = "this is index #LAITOS_CLIENTADDR #LAITOS_3339TIME"
)

// PrepareForTestHTTPD sets up a directory and HTML file to be hosted by HTTPD during tests.
func PrepareForTestHTTPD(t testingstub.T) {
	// Create a temporary file for index
	_ = os.MkdirAll("/tmp", 1777)
	indexFile := "/tmp/test-laitos-index.html"
	if err := os.WriteFile(indexFile, []byte(TestLaitosIndexHTMLContent), 0644); err != nil {
		panic(err)
	}
	htmlDir := "/tmp/test-laitos-dir"
	if err := os.MkdirAll(htmlDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(htmlDir+"/a.html", []byte("a html"), 0644); err != nil {
		t.Fatal(err)
	}
	/*
		Unfortunately due to the difficulty in making preparations for concurrent tests on multiple HTTP daemons, there
		won't be automated clean up for these files.
	*/
	// Globally enable prometheus integration so to ensure that initialisation and metrics recording code will run during the test
	misc.EnablePrometheusIntegration = true
}

// Run unit test on HTTP daemon. See TestHTTPD_StartAndBlock for daemon setup.
func TestHTTPD(httpd *Daemon, t testingstub.T) {
	addr := fmt.Sprintf("http://%s:%d", httpd.Address, httpd.Port)

	// Directory handle
	resp, err := inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/my/dir")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != `<!doctype html>
<meta name="viewport" content="width=device-width">
<pre>
<a href="a.html">a.html</a>
</pre>
` {
		t.Fatalf("%v\n%s\n%v", err, string(resp.Body), resp)
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/my/dir/a.html")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != "a html" {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/dir")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != `<!doctype html>
<meta name="viewport" content="width=device-width">
<pre>
<a href="a.html">a.html</a>
</pre>
` {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/dir/a.html")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != "a html" {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Non-existent path in directory
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/my/dir/doesnotexist.html")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/doesnotexist")
	// Non-existent path, but go is quite stupid that it produces response of /
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Test hitting rate limits
	time.Sleep(RateLimitIntervalSec * time.Second)
	success := 0
	for i := 0; i < httpd.PerIPLimit*DirectoryHandlerRateLimitFactor*2; i++ {
		resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/my/dir/a.html")
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
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, addr+"/my/dir")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != `<!doctype html>
<meta name="viewport" content="width=device-width">
<pre>
<a href="a.html">a.html</a>
</pre>
` {
		t.Fatal(err, string(resp.Body), resp)
	}
}
