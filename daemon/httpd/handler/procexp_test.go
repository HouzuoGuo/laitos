package handler

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"

	"github.com/HouzuoGuo/laitos/platform/procexp"
)

func TestHandleProcessExplorer_SelfTest(t *testing.T) {
	handler := &HandleProcessExplorer{}
	if err := handler.SelfTest(); err != nil {
		t.Fatal(err)
	}

}

func TestHandleProcessExplorer_Handle(t *testing.T) {
	handler := &HandleProcessExplorer{}
	t.Run("get all process IDs", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		handler.Handle(w, req)
		body, _ := ioutil.ReadAll(w.Result().Body)
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("%+v", w.Result())
		}
		var pids []int
		if err := json.Unmarshal(body, &pids); err != nil {
			t.Fatal(err)
		}
		if len(pids) < 2 {
			t.Fatalf("%+v", pids)
		}
		if idx := sort.SearchInts(pids, os.Getpid()); idx == len(pids) {
			t.Fatalf("%+v", pids)
		}
	})
	t.Run("get process status for its own PID", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/?pid=0", nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		handler.Handle(w, req)
		body, _ := ioutil.ReadAll(w.Result().Body)
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("%+v", w.Result())
		}
		var status procexp.ProcessStatus
		if err := json.Unmarshal(body, &status); err != nil {
			t.Fatal(err)
		}
		if status.ProcessID != os.Getpid() {
			t.Fatalf("%+v", status)
		}
	})
	t.Run("get process status for specified PID", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/?pid=1", nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		handler.Handle(w, req)
		body, _ := ioutil.ReadAll(w.Result().Body)
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("%+v", w.Result())
		}
		var status procexp.ProcessStatus
		if err := json.Unmarshal(body, &status); err != nil {
			t.Fatal(err)
		}
		if status.ProcessID != 1 {
			t.Fatalf("%+v", status)
		}
	})
}
