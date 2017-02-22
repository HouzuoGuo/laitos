package feature

import (
	"errors"
	"github.com/HouzuoGuo/websh/httpclient"
	"os"
	"testing"
)

func TestCommand_Trim(t *testing.T) {
	cmd := Command{Content: "\n\t   abc   \t\n"}
	if resultErr := cmd.Trim(); resultErr != nil {
		t.Fatal(resultErr)
	}
	if cmd.Content != "abc" {
		t.Fatal(cmd.Content)
	}

	cmd = Command{Content: "    \t\n     "}
	if resultErr := cmd.Trim(); resultErr == nil ||
		resultErr.Error != ErrEmptyCommand || resultErr.Output != "" {
		t.Fatal(resultErr)
	}
}

func TestResult(t *testing.T) {
	result := Result{Error: nil, Output: "abc"}
	if result.ErrText() != "" {
		t.Fatal(result.ErrText())
	}
	if result.CombinedText() != "abc" {
		t.Fatal(result.CombinedText())
	}

	result = Result{Error: os.ErrInvalid, Output: "abc"}
	if result.ErrText() != os.ErrInvalid.Error() {
		t.Fatal(result.ErrText())
	}
	if result.CombinedText() != os.ErrInvalid.Error()+COMBINED_TEXT_SEP+"abc" {
		t.Fatal(result.CombinedText())
	}
}

func TestHTTPErrorToResult(t *testing.T) {
	resp := httpclient.Response{StatusCode: 201}
	if result := HTTPErrorToResult(resp, nil); result != nil {
		t.Fatal(result)
	}
	if result := HTTPErrorToResult(resp, errors.New("an error")); result.Error == nil {
		t.Fatal("did not error")
	}
	resp.StatusCode = 400
	if result := HTTPErrorToResult(resp, nil); result.Error == nil {
		t.Fatal("did not error")
	}
	if result := HTTPErrorToResult(resp, errors.New("an error")); result.Error == nil {
		t.Fatal("did not error")
	}
}

func TestLog(t *testing.T) {
	// Log functions should not crash
	LogBeforeExecute(Command{})
	LogAfterExecute(Command{}, nil)
	LogAfterExecute(Command{}, &Result{})
}
