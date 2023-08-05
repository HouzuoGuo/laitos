package handler

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/httpd/middleware"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const (
	// HTMLCurrentDateTime is the string anchor to be replaced by current system time in rendered HTML output.
	HTMLCurrentDateTime = "#LAITOS_3339TIME"

	// HTMLClientAddress it the string anchor to be replaced by HTTP client IP address in rendered HTML output.
	HTMLClientAddress = "#LAITOS_CLIENTADDR"
)

// HandleHTMLDocument responds to the HTTP request with a static HTML page.
// The page may contain placeholders (for example "#LAITOS_3339TIME"), which
// will be substituted with rendered values when the page is served to a client.
type HandleHTMLDocument struct {
	// HTMLContent is the content of HTML page, which may contain magic
	// placeholders. It is OK to configure the handler to serve an empty HTML
	// page with no content at all.
	HTMLContent string `json:"HTMLContent"`
	// HTMLFilePath is the file path to the HTML page, the content of which may
	// contain placeholders.
	HTMLFilePath string `json:"HTMLFilePath"`

	contentBytes  []byte // contentBytes is the HTML document file's content in bytes
	contentString string // contentString is the HTML document file's content in string
}

func (doc *HandleHTMLDocument) Initialise(*lalog.Logger, *toolbox.CommandProcessor, string) error {
	var err error
	doc.contentString = doc.HTMLContent
	if doc.HTMLFilePath != "" {
		if doc.contentBytes, err = os.ReadFile(doc.HTMLFilePath); err != nil {
			return fmt.Errorf("HandleHTMLDocument.Initialise: failed to open HTML file at %s - %v", doc.HTMLFilePath, err)
		}
		doc.contentString = string(doc.contentBytes)
	}
	return nil
}

func (doc *HandleHTMLDocument) Handle(w http.ResponseWriter, r *http.Request) {
	// Inject browser client IP and current time into index document and return.
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	NoCache(w)
	page := strings.Replace(doc.contentString, HTMLCurrentDateTime, time.Now().Format(time.RFC3339), -1)
	page = strings.Replace(page, HTMLClientAddress, middleware.GetRealClientIP(r), -1)
	_, _ = w.Write([]byte(page))
}

func (_ *HandleHTMLDocument) GetRateLimitFactor() int {
	// Some laitos installations are web servers behind a load balancer, they
	// use an HTML document handler for the load balancer's health check
	// endpoint. In some rare cases (e.g. AWS Beanstalk using a combo of elastic
	// load balancer and on-instance nginx proxy) are very busy and make far
	// more requests than what was configured.
	// Therefore, be extra generous about the rate limit factor.
	return 8
}

func (_ *HandleHTMLDocument) SelfTest() error {
	return nil
}
