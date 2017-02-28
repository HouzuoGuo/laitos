package httpd

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/websh/frontend/common"
	"github.com/HouzuoGuo/websh/frontend/httpd/api"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

var IndexLocations = []string{"/", "/index", "/index.htm", "/index.html", "/index.php"} // Locations to which index document is served

// Return true if input character is a forward ot backward slash.
func IsSlash(c rune) bool {
	return c == '\\' || c == '/'
}

/*
Create a handler func that serves all of the input routes.
Input routes must use forward slash in URL.
This function exists to work around Go's inability to serve an independent handler on /.
*/
func MakeRootHandlerFunc(allRoutes map[string]http.HandlerFunc) http.HandlerFunc {
	maxURLFields := 0
	for urlLocation := range allRoutes {
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
		// Look up the partial URL to find handler function
		if fun, found := allRoutes[assembledPath]; found {
			log.Printf("HTTPD: Handle %s - %s - %s", r.RemoteAddr, r.Method, assembledPath)
			fun(w, r)
		} else {
			log.Printf("HTTPD: NotFound %s - %s - %s", r.RemoteAddr, r.Method, assembledPath)
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}
}

// An HTTP daemon.
type HTTPD struct {
	ListenAddress      string                        `json:"ListenAddress"`      // Network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	ListenPort         int                           `json:"ListenPort"`         // Port number to listen on
	TLSCertPath        string                        `json:"TLSCertPath"`        // (Optional) serve HTTPS via this certificate
	TLSKeyPath         string                        `json:"TLSKeyPath"`         // (Optional) serve HTTPS via this certificate (key)
	ServeDirectories   map[string]string             `json:"ServeDirectories"`   // Serve directories (value) on prefix paths (key)
	ServeIndexDocument string                        `json:"ServeIndexDocument"` // Serve this HTML document as index document
	IndexContent       []byte                        `json:"-"`                  // The content of ServeIndexDocument file is read into memory
	SpecialHandlers    map[string]api.HandlerFactory `json:"-"`                  // Specialised handlers that implement api.HandlerFactory interface
	AllRoutes          map[string]http.HandlerFunc   `json:"-"`                  // Aggregate all routes from all handlers
	Server             *http.Server                  `json:"-"`                  // Standard library HTTP server structure
	Processor          *common.CommandProcessor      `json:"-"`                  // Common command processor
}

// Check configuration and initialise internal states.
func (httpd *HTTPD) Initialise() error {
	if httpd.ListenAddress == "" {
		return errors.New("HTTPD.StartAndBlock: listen address is empty")
	}
	if httpd.ListenPort == 0 {
		return errors.New("HTTPD.StartAndBlock: listen port must not be empty or 0")
	}
	if (httpd.TLSCertPath != "" || httpd.TLSKeyPath != "") && (httpd.TLSCertPath == "" || httpd.TLSKeyPath == "") {
		return errors.New("HTTPD.StartAndBlock: if TLS is to be enabled, both TLS certificate and key path must be present.")
	}
	if errs := httpd.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("HTTPD.StartAndBlock: %+v", errs)
	}
	// Work around Go's inability to serve a handler on / and only /
	httpd.AllRoutes = map[string]http.HandlerFunc{}
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
		}
	}
	// Collect specialised handlers
	for urlLocation, handler := range httpd.SpecialHandlers {
		fun, err := handler.MakeHandler(httpd.Processor)
		if err != nil {
			return err
		}
		httpd.AllRoutes[urlLocation] = fun
	}
	// Collect index handlers
	if httpd.ServeIndexDocument != "" {
		var err error
		if httpd.IndexContent, err = ioutil.ReadFile(httpd.ServeIndexDocument); err != nil {
			return fmt.Errorf("HTTPD.StartAndBlock: failed to open index document file - %v", err)
		}
		indexHandler := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(httpd.IndexContent)
		}
		for _, urlLocation := range IndexLocations {
			httpd.AllRoutes[urlLocation] = indexHandler
		}
	}
	// Install the handlers to /
	rootHandler := MakeRootHandlerFunc(httpd.AllRoutes)
	muxHandlers := http.NewServeMux()
	muxHandlers.HandleFunc("/", rootHandler)
	// Configure server with rather generous and sane defaults
	httpd.Server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", httpd.ListenAddress, httpd.ListenPort),
		Handler:      muxHandlers,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}
	return nil
}

/*
You may only call this function after having called Initialise()
Start HTTP daemon and block until this program exits.
*/
func (httpd *HTTPD) StartAndBlock() error {
	if httpd.TLSCertPath == "" {
		log.Printf("HTTPD.StartAndBlock: will listen for HTTPS traffic on %s:%d", httpd.ListenAddress, httpd.ListenPort)
		return httpd.Server.ListenAndServe()
	} else {
		log.Printf("HTTPD.StartAndBlock: will listen for HTTP traffic on %s:%d", httpd.ListenAddress, httpd.ListenPort)
		return httpd.Server.ListenAndServeTLS(httpd.TLSCertPath, httpd.TLSKeyPath)
	}
	// Never reached
	return nil
}
