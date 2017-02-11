package feature

import (
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

func TestHTTPResponseError(t *testing.T) {
	if errResult := HTTPResponseError(200, []byte("good"), nil); errResult != nil {
		t.Fatal(errResult)
	}
	if errResult := HTTPResponseError(400, []byte("bad"), nil); errResult == nil ||
		errResult.Error.Error() != "HTTP 400: bad" ||
		errResult.Output != "bad" {
		t.Fatal(errResult)
	}
	if errResult := HTTPResponseError(200, []byte("also bad"), os.ErrInvalid); errResult == nil ||
		errResult.Error != os.ErrInvalid ||
		errResult.Output != "also bad" {
		t.Fatal(errResult)
	}
}

func TestLog(t *testing.T) {
	// Log functions should not crash
	LogBeforeExecute(&Command{})
	LogAfterExecute(&Command{}, nil)
	LogAfterExecute(&Command{}, &Result{})
}
