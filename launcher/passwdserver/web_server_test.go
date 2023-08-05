package passwdserver

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/autounlock"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
)

func TestWebServer(t *testing.T) {
	ws := WebServer{
		Port: 54396,
		URL:  "/test-url",
	}
	// Shutting down a server not yet started should have no negative effect
	ws.Shutdown()
	shutdownChan := make(chan struct{}, 1)
	go func() {
		if err := ws.Start(); err != nil {
			t.Error(err)
			return
		}
		shutdownChan <- struct{}{}
	}()
	// Expect server to start within a second
	if !misc.ProbePort(30*time.Second, "localhost", ws.Port) {
		t.Fatal("server did not start in time")
	}
	resp, err := inet.DoHTTP(context.Background(), inet.HTTPRequest{}, fmt.Sprintf("http://localhost:%d", ws.Port))
	// Visit any URL but the correct URL will return empty body
	if err != nil || string(resp.Body) != "" {
		t.Fatal(err, string(resp.Body))
	}
	// Visit the correct URL for password unlock page
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{}, fmt.Sprintf("http://localhost:%d%s", ws.Port, ws.URL))
	if err != nil || !strings.Contains(string(resp.Body), "Clock") || !strings.Contains(string(resp.Body), "Enter password") {
		t.Fatal(string(resp.Body))
	}
	// Shutdown the server
	ws.Shutdown()
	<-shutdownChan
	// Repeatedly shutting down the server should have no negative effect
	ws.Shutdown()
}

func TestWebServer_UnlockWithPassword(t *testing.T) {
	// Create an encrypted config file for testing
	file, err := os.CreateTemp("", "laitos-TestWebServer")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(file.Name())
	misc.ConfigFilePath = file.Name()
	if err := os.WriteFile(file.Name(), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	encPassword := "obtain_password_test"
	if err := misc.Encrypt(file.Name(), encPassword); err != nil {
		t.Fatal(err)
	}
	// Start the web server
	ws := WebServer{
		Port: 44511,
		URL:  "/test-url",
	}
	defer ws.Shutdown()
	go func() {
		if err := ws.Start(); err != nil {
			t.Error(err)
			return
		}
	}()
	if !misc.ProbePort(30*time.Second, "localhost", ws.Port) {
		t.Fatal("server did not start in time")
	}
	// Give it an incorrect password and expect an error response
	resp, err := inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{autounlock.PasswordInputName: []string{"wrong-password"}}.Encode()),
	}, fmt.Sprintf("http://localhost:%d%s", ws.Port, ws.URL))
	if err != nil || !strings.Contains(string(resp.Body), "wrong key") {
		t.Fatal(string(resp.Body))
	}
	// Give it the correct password and read it back from collected password channel
	resp, err = inet.DoHTTP(context.Background(), inet.HTTPRequest{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{autounlock.PasswordInputName: []string{encPassword}}.Encode()),
	}, fmt.Sprintf("http://localhost:%d%s", ws.Port, ws.URL))
	if err != nil || !strings.Contains(string(resp.Body), "success") {
		t.Fatal(string(resp.Body))
	}
	collectedPassword := <-misc.ProgramDataDecryptionPasswordInput
	if collectedPassword != encPassword {
		t.Fatalf("incorrect collected password: %s", collectedPassword)
	}
}
