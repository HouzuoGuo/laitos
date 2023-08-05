package handler

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/lalog"
)

func TestRequestInspector(t *testing.T) {
	handler := &HandleRequestInspector{}
	if err := handler.Initialise(&lalog.Logger{}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := handler.SelfTest(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodOptions, "/requrl", strings.NewReader("request-body"))
	req.Header.Add("custom-header", "custom-header-value")
	req.Host = "custom-host"
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.Handle(w, req)
	body, _ := ioutil.ReadAll(w.Result().Body)
	bodyStr := string(body)
	t.Log(bodyStr)

	for _, needle := range []string{"requrl", "request-body", "custom-header", "custom-header-value", "custom-host"} {
		if !strings.Contains(bodyStr, needle) {
			t.Fatal("cannot find needle", needle)
		}
	}
}
