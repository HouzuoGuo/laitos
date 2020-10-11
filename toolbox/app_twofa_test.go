package toolbox

import (
	"context"
	"strings"
	"testing"
)

func TestGetTwoFACodeForTimeDivision(t *testing.T) {
	if result, err := GetTwoFACodeForTimeDivision("iuu3xchz3ftf6hdh", 49943698); err != nil || result != "642882" {
		t.Fatal(result, err)
	}
}

func TestTwoFACodeGenerator_Execute(t *testing.T) {
	// Prepare feature using incorrect configuration should result in error
	codegen := TwoFACodeGenerator{}
	if codegen.IsConfigured() {
		t.Fatal("not right")
	}
	codegen.SecretFile = &AESEncryptedFile{
		FilePath:     "",
		HexIV:        "",
		HexKeyPrefix: "",
	}
	if err := codegen.Initialise(); err == nil {
		t.Fatal("did not error")
	}
	codegen.SecretFile = &AESEncryptedFile{
		FilePath:     "this file does not exist",
		HexIV:        "",
		HexKeyPrefix: "",
	}
	if err := codegen.Initialise(); err == nil {
		t.Fatal("did not error")
	}
	if err := codegen.SelfTest(); err == nil {
		t.Fatal("did not error")
	}
	// Prepare a good decrypt
	codegen = GetTestTwoFACodeGenerator()
	if !codegen.IsConfigured() {
		t.Fatal("not right")
	}
	if err := codegen.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := codegen.SelfTest(); err != nil {
		t.Fatal(err)
	}
	// Bad parameter
	if ret := codegen.Execute(context.Background(), Command{TimeoutSec: 10, Content: "haha"}); ret.Error != ErrBadTwoFAParam {
		t.Fatal("did not error")
	}
	// Specify non-existent account
	if ret := codegen.Execute(context.Background(), Command{TimeoutSec: 10, Content: "5512 does not exist"}); !strings.HasPrefix(ret.Error.Error(), "Cannot find the account") {
		t.Fatal(ret)
	}
	// Specify bad key
	if ret := codegen.Execute(context.Background(), Command{TimeoutSec: 10, Content: "beef test"}); !strings.HasPrefix(ret.Error.Error(), "Cannot find the account") {
		t.Fatal(ret)
	}
	// Get codes using good parameters
	if ret := codegen.Execute(context.Background(), Command{TimeoutSec: 10, Content: "5512 test acc"}); ret.Error != nil || !strings.HasPrefix(ret.Output, "test account: ") {
		t.Fatal(ret)
	}
}
