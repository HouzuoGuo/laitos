package handler

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HouzuoGuo/laitos/daemon/httpd/middleware"
	"github.com/HouzuoGuo/laitos/lalog"
)

func TestHandleLatestRequestsInspector(t *testing.T) {
	handler := &HandleLatestRequestsInspector{}
	if err := handler.Initialise(&lalog.Logger{}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := handler.SelfTest(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodOptions, "/?e=true", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.Handle(w, req)
	body, _ := ioutil.ReadAll(w.Result().Body)
	bodyStr := string(body)
	if bodyStr != "Start memorising latest requests." {
		t.Fatal(bodyStr)
	}

	req, err = http.NewRequest(http.MethodOptions, "/?e=0", nil)
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	handler.Handle(w, req)
	body, _ = ioutil.ReadAll(w.Result().Body)
	bodyStr = string(body)
	if bodyStr != "Stop memorising latest requests." {
		t.Fatal(bodyStr)
	}

	middleware.LatestRequests.Push("req1")
	middleware.LatestRequests.Push("req2")
	middleware.LatestRequests.Push("req3")
	req, err = http.NewRequest(http.MethodOptions, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	handler.Handle(w, req)
	body, _ = ioutil.ReadAll(w.Result().Body)
	bodyStr = string(body)
	if bodyStr != "<pre>req1</pre><hr>\n<pre>req2</pre><hr>\n<pre>req3</pre><hr>\n" {
		t.Fatal(bodyStr)
	}
}
