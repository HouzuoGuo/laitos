package api

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/misc"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// Render an HTML page that has client IP and current time injected into the content.
type HandleHTMLDocument struct {
	HTMLFilePath string `json:"HTMLFilePath"`
}

func (index *HandleHTMLDocument) MakeHandler(logger misc.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	var err error
	var contentBytes []byte
	if contentBytes, err = ioutil.ReadFile(index.HTMLFilePath); err != nil {
		return nil, fmt.Errorf("HandleHTMLDocument.MakeHandler: failed to open HTML file at %s - %v", index.HTMLFilePath, err)
	}
	contentStr := string(contentBytes)
	// Inject browser client IP and current time into index document and return.
	fun := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		NoCache(w)
		page := strings.Replace(contentStr, "#LAITOS_3339TIME", time.Now().Format(time.RFC3339), -1)
		page = strings.Replace(page, "#LAITOS_CLIENTADDR", GetRealClientIP(r), -1)
		w.Write([]byte(page))
	}
	return fun, nil
}

func (index *HandleHTMLDocument) GetRateLimitFactor() int {
	return 25
}
