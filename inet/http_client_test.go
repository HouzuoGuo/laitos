package inet

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
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
	// Create a test server that serves 5 bad responses and HTTP 201 in subsequent responses
	faultyServerRequestsServed := 0
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	router := http.NewServeMux()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		faultyServerRequestsServed++
		switch faultyServerRequestsServed {
		case 1:
			http.Error(w, "haha", 401)
		case 2:
			http.Error(w, "hehe", 501)
		case 3:
			http.Error(w, "hihi", 402)
		case 4:
			http.Error(w, "hoho", 502)
		case 5:
			http.Error(w, "hfhf", 403)
		default:
			// Further responses are HTTP 201
			w.WriteHeader(201)
			w.Write([]byte("response from faulty server"))
		}
	})
	go http.Serve(listener, router)
	// Expect server to be ready in a second
	time.Sleep(1 * time.Second)

	// Expect first request to fail in all three attempts
	serverURL := fmt.Sprintf("http://localhost:%d/endpoint", listener.Addr().(*net.TCPAddr).Port)
	resp, err := DoHTTP(HTTPRequest{}, serverURL)
	if err != nil || string(resp.Body) != "hihi\n" || resp.StatusCode != 402 {
		t.Fatal(err, string(resp.Body), resp.StatusCode)
	}
	if faultyServerRequestsServed != 3 {
		t.Fatal(faultyServerRequestsServed)
	}

	// Expect the next request to succeed in three attempts
	resp, err = DoHTTP(HTTPRequest{}, serverURL)
	if err != nil || string(resp.Body) != "response from faulty server" || resp.StatusCode != 201 {
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
		w.Write([]byte("response from ok server"))
	})
	go http.Serve(listener, router)
	// Expect server to be ready in a second
	time.Sleep(1 * time.Second)

	// Expect to make exactly one request against the good HTTP server
	serverURL := fmt.Sprintf("http://localhost:%d/endpoint", listener.Addr().(*net.TCPAddr).Port)
	resp, err := DoHTTP(HTTPRequest{}, serverURL)
	if err != nil || string(resp.Body) != "response from ok server" || resp.StatusCode != 202 {
		t.Fatal(err, string(resp.Body), resp.StatusCode)
	}
	if okServerRequestsServed != 1 {
		t.Fatal(okServerRequestsServed)
	}
}

func TestDoHTTPPublicServer(t *testing.T) {
	resp, err := DoHTTP(HTTPRequest{
		TimeoutSec: 30,
	}, "https://this-name-does-not-exist-aewifnvjnjfdfdozoio.rich")
	if err == nil {
		t.Fatal("did not error")
	}
	if resp.Non2xxToError() == nil {
		t.Fatal("did not error")
	}

	resp, err = DoHTTP(HTTPRequest{
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
