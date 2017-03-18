package httpd

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/frontend/httpd/api"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/ratelimit"
	"net/http"
	"strings"
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
	Logger          lalog.Logger                    `json:"-"` // Logger
}

// Check configuration and initialise internal states.
func (httpd *HTTPD) Initialise() error {
	if errs := httpd.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("HTTPD.Initialise: %+v", errs)
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
				httpd.Logger.Printf("Handle", remoteIP, nil, "NotFound %s %s", r.Method, assembledPath)
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
