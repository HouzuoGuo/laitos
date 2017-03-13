package feature

import (
	"errors"
	"github.com/HouzuoGuo/laitos/httpclient"
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

func TestCommand_FindAndRemovePrefix(t *testing.T) {
	original := "abc\n\t   def   \n\t"
	cmd := Command{Content: original}
	if cmd.FindAndRemovePrefix("not a prefix") {
		t.Fatal("should not remove")
	}
	if cmd.Content != original {
		t.Fatal(cmd.Content)
	}
	if !cmd.FindAndRemovePrefix("abc") {
		t.Fatal("did not remove")
	}
	if cmd.Content != "def" {
		t.Fatal(cmd.Content)
	}
}

func TestResult(t *testing.T) {
	result := Result{Error: nil, Output: "abc"}
	if result.ErrText() != "" {
		t.Fatal(result.ErrText())
	}
	if result.ResetCombinedText() != "abc" {
		t.Fatal(result.ResetCombinedText())
	}

	result = Result{Error: os.ErrInvalid, Output: "abc"}
	if result.ErrText() != os.ErrInvalid.Error() {
		t.Fatal(result.ErrText())
	}
	if result.ResetCombinedText() != os.ErrInvalid.Error()+CombinedTextSeparator+"abc" {
		t.Fatal(result.ResetCombinedText())
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
