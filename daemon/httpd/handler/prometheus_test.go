package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

func TestHandlePrometheus_SelfTest(t *testing.T) {
	misc.EnablePrometheusIntegration = true
	handler := &HandlePrometheus{}
	if err := handler.Initialise(&lalog.Logger{}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := handler.SelfTest(); err != nil {
		t.Fatal(err)
	}
}

func TestHandlePrometheus_HandleWithPromIntegDisabled(t *testing.T) {
	misc.EnablePrometheusIntegration = false
	handler := &HandlePrometheus{}
	if err := handler.Initialise(&lalog.Logger{}, nil, ""); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodGet, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.Handle(w, req)
	if w.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("%+v", w.Result())
	}
}

func TestHandlePrometheus_HandleWithPromIntegEnabled(t *testing.T) {
	misc.EnablePrometheusIntegration = true
	handler := &HandlePrometheus{}
	if err := handler.Initialise(&lalog.Logger{}, nil, ""); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodGet, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.Handle(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("%+v", w.Result())
	}
	body, _ := io.ReadAll(w.Result().Body)
	if !strings.Contains(string(body), "go_memstats_heap_objects") {
		t.Fatalf("missing metrics readings from response body: %s", string(body))
	}
}
