package inet

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/misc"
)

func TestHTTPRequest_FillBlanks(t *testing.T) {
	req := HTTPRequest{}
	req.FillBlanks()
	if req.TimeoutSec != 30 {
		t.Fatal(req.TimeoutSec)
	}
	if req.Method != "GET" {
		t.Fatal(req.Method)
	}
	if req.ContentType != "application/x-www-form-urlencoded" {
		t.Fatal(req.ContentType)
	}
	if req.MaxBytes != 4*1048576 {
		t.Fatal(req.MaxBytes)
	}
	if req.MaxRetry != 3 {
		t.Fatal(req.MaxRetry)
	}

	req = HTTPRequest{
		TimeoutSec:  123,
		Method:      "POST",
		ContentType: "application/json",
		MaxBytes:    456,
		MaxRetry:    789,
	}
	req.FillBlanks()
	if req.TimeoutSec != 123 {
		t.Fatal(req.TimeoutSec)
	}
	if req.Method != "POST" {
		t.Fatal(req.Method)
	}
	if req.ContentType != "application/json" {
		t.Fatal(req.ContentType)
	}
	if req.MaxBytes != 456 {
		t.Fatal(req.MaxBytes)
	}
	if req.MaxRetry != 789 {
		t.Fatal(req.MaxRetry)
	}
}

func TestDoHTTPFaultyServer(t *testing.T) {
	faultyServerRequestsServed := 0
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	router := http.NewServeMux()
	requestBodyMatch := []byte("request body match string")
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// The handler serves exactly 5 requests with non-200 response codes
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		if !bytes.Equal(reqBody, requestBodyMatch) {
			t.Error("incorrect request body", string(reqBody))
			return
		}
		faultyServerRequestsServed++
		switch faultyServerRequestsServed {
		case 1:
			http.Error(w, "haha", http.StatusUnauthorized)
		case 2:
			http.Error(w, "hehe", http.StatusNotImplemented)
		case 3:
			http.Error(w, "hihi", http.StatusPaymentRequired)
		case 4:
			http.Error(w, "hoho", http.StatusBadGateway)
		case 5:
			http.Error(w, "hfhf", http.StatusForbidden)
		default:
			// All subsequent requests get a 201 response
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("response from faulty server"))
		}
	})
	go func() {
		if err = http.Serve(listener, router); err != nil {
			t.Error(err)
			return
		}
	}()
	if !misc.ProbePort(30*time.Second, "localhost", listener.Addr().(*net.TCPAddr).Port) {
		t.Fatal("server did not start in time")
	}

	// Expect first request to fail in all three attempts
	serverURL := fmt.Sprintf("http://localhost:%d/endpoint", listener.Addr().(*net.TCPAddr).Port)
	resp, err := DoHTTP(context.Background(), HTTPRequest{Body: bytes.NewReader(requestBodyMatch)}, serverURL)
	if err != nil || string(resp.Body) != "hihi\n" || resp.StatusCode != http.StatusPaymentRequired {
		t.Fatal(err, string(resp.Body), resp.StatusCode)
	}
	if faultyServerRequestsServed != 3 {
		t.Fatal(faultyServerRequestsServed)
	}

	// Expect the next request to succeed in three attempts
	resp, err = DoHTTP(context.Background(), HTTPRequest{Body: bytes.NewReader(requestBodyMatch)}, serverURL)
	if err != nil || string(resp.Body) != "response from faulty server" || resp.StatusCode != http.StatusCreated {
		t.Fatal(err, string(resp.Body), resp.StatusCode)
	}
	if faultyServerRequestsServed != 6 {
		t.Fatal(faultyServerRequestsServed)
	}
}

func TestDoHTTPGoodServer(t *testing.T) {
	// Create a testserver that always gives successful HTTP 202 response
	okServerRequestsServed := 0
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	router := http.NewServeMux()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		okServerRequestsServed++
		w.WriteHeader(202)
		_, _ = w.Write([]byte("response from ok server"))
	})
	go func() {
		if err := http.Serve(listener, router); err != nil {
			t.Error(err)
			return
		}
	}()
	if !misc.ProbePort(30*time.Second, "localhost", listener.Addr().(*net.TCPAddr).Port) {
		t.Fatal("server did not start in time")
	}

	// Expect to make exactly one request against the good HTTP server
	serverURL := fmt.Sprintf("http://localhost:%d/endpoint", listener.Addr().(*net.TCPAddr).Port)
	resp, err := DoHTTP(context.Background(), HTTPRequest{}, serverURL)
	if err != nil || string(resp.Body) != "response from ok server" || resp.StatusCode != 202 {
		t.Fatal(err, string(resp.Body), resp.StatusCode)
	}
	if okServerRequestsServed != 1 {
		t.Fatal(okServerRequestsServed)
	}
}

func TestDoHTTPPublicServer(t *testing.T) {
	resp, err := DoHTTP(context.Background(), HTTPRequest{
		TimeoutSec: 30,
	}, "https://this-name-does-not-exist-aewifnvjnjfdfdozoio.rich")
	if err == nil {
		t.Fatal("did not error")
	}
	if resp.Non2xxToError() == nil {
		t.Fatal("did not error")
	}

	resp, err = DoHTTP(context.Background(), HTTPRequest{
		TimeoutSec: 30,
	}, "https://github.com")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Non2xxToError() != nil {
		t.Fatal(err)
	}
	if body := resp.GetBodyUpTo(10); len(body) != 10 {
		t.Fatal(body)
	}
}

func TestNeutralRecursiveResolver(t *testing.T) {
	// The timeout is applied to all resolution attempts
	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Duration(60*time.Second))
	defer cancel()
	for _, name := range []string{"apple.com", "github.com", "google.com", "microsoft.com", "wikipedia.org"} {
		for i := 0; i < 30; i++ {
			addrs, err := NeutralRecursiveResolver.LookupIPAddr(timeoutCtx, name)
			if err != nil {
				t.Fatalf("failed to resolve %s: %v", name, err)
			}
			if len(addrs) < 1 {
				t.Fatalf("name %s did not resolve to anything", name)
			}
		}
	}
}
