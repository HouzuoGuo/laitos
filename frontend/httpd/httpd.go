package httpd

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/websh/frontend/common"
	"github.com/HouzuoGuo/websh/frontend/httpd/api"
	"log"
	"net/http"
	"time"
)

// An HTTP daemon.
type HTTPD struct {
	ListenAddress string                        `json:"ListenAddress"`
	ListenPort    int                           `json:"ListenPort"`
	TLSCertPath   string                        `json:"TLSCertPath"`
	TLSKeyPath    string                        `json:"TLSKeyPath"`
	Handlers      map[string]api.HandlerFactory `json:"-"`
	Server        *http.Server                  `json:"-"`
	Processor     *common.CommandProcessor      `json:"-"`
}

// Start HTTP daemon and block until this program exits.
func (httpd *HTTPD) StartAndBlock() error {
	if httpd.ListenAddress == "" {
		return errors.New("Listen address is empty")
	}
	if httpd.ListenPort == 0 {
		return errors.New("Listen port must not be empty or 0")
	}
	if (httpd.TLSCertPath != "" || httpd.TLSKeyPath != "") && (httpd.TLSCertPath == "" || httpd.TLSKeyPath == "") {
		return errors.New("If TLS is to be enabled, both TLS certificate and key path must be present.")
	}
	if errs := httpd.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("%+v", errs)
	}
	// Install all handlers
	muxHandlers := http.NewServeMux()
	for url, handler := range httpd.Handlers {
		fun, err := handler.MakeHandler(httpd.Processor)
		if err != nil {
			return err
		}
		muxHandlers.HandleFunc(url, fun)
	}
	// Configure server with rather generous and sane defaults
	httpd.Server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", httpd.ListenAddress, httpd.ListenPort),
		Handler:      muxHandlers,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}
	if httpd.TLSCertPath == "" {
		log.Printf("HTTPD.StartAndBlock: will listen for HTTPS traffic on %s:%d", httpd.ListenAddress, httpd.ListenPort)
		return httpd.Server.ListenAndServe()
	} else {
		log.Printf("HTTPD.StartAndBlock: will listen for HTTP traffic on %s:%d", httpd.ListenAddress, httpd.ListenPort)
		return httpd.Server.ListenAndServeTLS(httpd.TLSCertPath, httpd.TLSKeyPath)
	}
}
