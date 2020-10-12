package lambda

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/misc"
)

func TestLambdaHandler(t *testing.T) {
	// Start a demo web server
	var lastHeaders http.Header
	var lastPath, lastQuery string
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		lastHeaders = req.Header
		lastPath = req.URL.Path
		lastQuery = req.URL.RawQuery
		w.Header().Set("X-Custom-Header", "header-value")
		_, _ = io.WriteString(w, "b")
	})
	go func() {
		if err := http.ListenAndServe("localhost:60110", nil); err != nil {
			panic(err)
		}
	}()
	// Assume that the demo web server starts in a second
	time.Sleep(1 * time.Second)

	// Invoke lambda handler to reach the local web server
	hand := Handler{}
	invocationInput := InvocationInput{
		StageVariables: map[string]string{
			"Test-Stage-Variable":      "val1",
			DecryptionPasswordStageVar: "test-password",
		},
		RequestContext: RequestContext{
			Path:       "/stage-dev/resource1",
			HTTPMethod: "DELETE",
			Stage:      "dev",
		},
		MultiValueQueryStringParameters: map[string][]string{
			"i": []string{"1"},
			"j": []string{"2"},
		},
		MultiValueHeaders: map[string][]string{
			"X-Head1": []string{"h1"},
			"X-Head2": []string{"h2"},
		},
		IsBase64Encoded: true,
		Body:            "YQ==", // "a"
	}
	input, err := json.Marshal(invocationInput)
	if err != nil {
		t.Fatal(err)
	}
	// Collect the program data decryption password
	var decryptionPassword string
	go func() {
		decryptionPassword = <-misc.ProgramDataDecryptionPasswordInput
	}()
	// Decode and handle the request
	lambdaResponse, err := hand.decodeAndHandleHTTPRequest("test-request-id", input, 60110)
	if err != nil {
		t.Fatal(err)
	}
	var invocationOutput InvocationOutput
	if err = json.Unmarshal(lambdaResponse, &invocationOutput); err != nil {
		t.Fatal(err)
	}
	// Check environment variables
	if s := os.Getenv("PORT"); s != strconv.Itoa(UpstreamWebServerPort) {
		t.Fatal(s)
	}
	// Check data decryption password
	if !IsProgramDataDecrypted {
		t.Fatal("did not decrypt program data")
	}
	if decryptionPassword != "test-password" {
		t.Fatal(decryptionPassword)
	}
	// Check web request translation
	if lastPath != "/stage-dev/resource1" {
		t.Fatal(lastPath)
	}
	if lastQuery != "i=1&j=2" {
		t.Fatal(lastQuery)
	}
	if !reflect.DeepEqual(lastHeaders["X-Head1"], []string{"h1"}) || !reflect.DeepEqual(lastHeaders["X-Head2"], []string{"h2"}) {
		t.Fatalf("%+v", lastHeaders)
	}
	// Check HTTP response
	if invocationOutput.StatusCode != http.StatusOK || !invocationOutput.IsBase64Encoded ||
		invocationOutput.Headers["X-Custom-Header"] != "header-value" || invocationOutput.Body != "Yg==" {
		t.Fatalf("%+v", invocationOutput)
	}
}
