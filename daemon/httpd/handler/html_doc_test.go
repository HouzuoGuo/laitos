package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

func TestHandleHTMLDocument_Empty(t *testing.T) {
	handler := &HandleHTMLDocument{}
	// Serving an empty HTML page.
	if err := handler.Initialise(&lalog.Logger{}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := handler.SelfTest(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodGet, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.Handle(w, req)
	body, err := io.ReadAll(w.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 0 {
		t.Fatal(string(body))
	}
}

func TestHandleHTMLDocument_HTMLContent(t *testing.T) {
	handler := &HandleHTMLDocument{HTMLContent: "test-anchor #LAITOS_3339TIME beginaddr#LAITOS_CLIENTADDRendaddr"}
	if err := handler.Initialise(&lalog.Logger{}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := handler.SelfTest(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodGet, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.Handle(w, req)
	body, err := io.ReadAll(w.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	bodyStr := string(body)
	for _, needle := range []string{"test-anchor", "beginaddrendaddr", time.Now().Format("2006")} {
		if !strings.Contains(string(body), needle) {
			t.Fatal(bodyStr)
		}
	}
}

func TestHandleHTMLDocument_HTMLFile(t *testing.T) {
	file, err := os.CreateTemp("", "laitos-handle-html-document")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(file.Name())
	if err := os.WriteFile(file.Name(), []byte("test-anchor #LAITOS_3339TIME beginaddr#LAITOS_CLIENTADDRendaddr"), 0644); err != nil {
		t.Fatal(err)
	}
	handler := &HandleHTMLDocument{HTMLFilePath: file.Name()}
	if err := handler.Initialise(&lalog.Logger{}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := handler.SelfTest(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodGet, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.Handle(w, req)
	body, err := io.ReadAll(w.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	bodyStr := string(body)
	for _, needle := range []string{"test-anchor", "beginaddrendaddr", time.Now().Format("2006")} {
		if !strings.Contains(string(body), needle) {
			t.Fatal(bodyStr)
		}
	}
}
