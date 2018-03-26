package handler

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/misc"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

const (
	// HTMLCurrentDateTime is the string anchor to be replaced by current system time in rendered HTML output.
	HTMLCurrentDateTime = "#LAITOS_3339TIME"

	// HTMLClientAddress it the string anchor to be replaced by HTTP client IP address in rendered HTML output.
	HTMLClientAddress = "#LAITOS_CLIENTADDR"
)

// HandleHTMLDocument renders an HTML page with client IP and current system time injected inside.
type HandleHTMLDocument struct {
	HTMLFilePath string `json:"HTMLFilePath"`

	contentBytes  []byte // contentBytes is the HTML document file's content in bytes
	contentString string // contentString is the HTML document file's content in string
}

func (doc *HandleHTMLDocument) Initialise(misc.Logger, *common.CommandProcessor) error {
	var err error
	if doc.contentBytes, err = ioutil.ReadFile(doc.HTMLFilePath); err != nil {
		return fmt.Errorf("HandleHTMLDocument.Initialise: failed to open HTML file at %s - %v", doc.HTMLFilePath, err)
	}
	doc.contentString = string(doc.contentBytes)
	return nil
}

func (doc *HandleHTMLDocument) Handle(w http.ResponseWriter, r *http.Request) {
	// Inject browser client IP and current time into index document and return.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	NoCache(w)
	page := strings.Replace(doc.contentString, HTMLCurrentDateTime, time.Now().Format(time.RFC3339), -1)
	page = strings.Replace(page, HTMLClientAddress, GetRealClientIP(r), -1)
	w.Write([]byte(page))
}

func (_ *HandleHTMLDocument) GetRateLimitFactor() int {
	/*
		Usually nobody visits the index page (or plain HTML document) this often, but on Elastic Beanstalk the nginx
		proxy in front of the HTTP 80 server visits the index page a lot! If HTTP server fails to serve this page,
		Elastic Beanstalk will consider the instance unhealthy. Therefore, the factor here allows 4x as many requests
		to be processed.
	*/
	return 8
}

func (_ *HandleHTMLDocument) SelfTest() error {
	return nil
}
