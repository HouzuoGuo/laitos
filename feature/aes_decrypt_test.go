package feature

import (
	"strings"
	"testing"
)

func TestAESDecrypt_Execute(t *testing.T) {
	// Prepare feature using incorrect configuration should result in error
	decrypt := AESDecrypt{}
	if decrypt.IsConfigured() {
		t.Fatal("not right")
	}
	decrypt.EncryptedFiles = map[string]*AESEncryptedFile{
		"alpha": {
			FilePath:     "",
			HexIV:        "",
			HexKeyPrefix: "",
		},
	}
	if err := decrypt.Initialise(); err == nil {
		t.Fatal("did not error")
	}
	decrypt.EncryptedFiles = map[string]*AESEncryptedFile{
		"alpha": &AESEncryptedFile{
			FilePath:     "this file does not exist",
			HexIV:        "A28DB439E2D112AB6E9FC2B09A73B605",
			HexKeyPrefix: "A28DB439E2D112AB6E9FC2B09A73B605",
		},
	}
	if err := decrypt.Initialise(); err == nil {
		t.Fatal("did not error")
	}
	if err := decrypt.SelfTest(); err == nil {
		t.Fatal("did not error")
	}
	// Prepare a good decrypt
	decrypt = GetTestAESDecrypt()
	if !decrypt.IsConfigured() {
		t.Fatal("not right")
	}
	if err := decrypt.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := decrypt.SelfTest(); err != nil {
		t.Fatal(err)
	}
	// Decrypt but parameters aren't given
	if ret := decrypt.Execute(Command{TimeoutSec: 10, Content: "haha hoho"}); ret.Error != ErrBadAESDecryptParam {
		t.Fatal("did not error")
	}
	// Decrypt unregistered file
	if ret := decrypt.Execute(Command{TimeoutSec: 10, Content: "charlie 0000 0000"}); !strings.HasPrefix(ret.Error.Error(), "Cannot find") {
		t.Fatal(ret)
	}
	// Decrypt file using bad key
	// (The key accidentally decrypts into Re0b, so don't use them to test content search)
	if ret := decrypt.Execute(Command{TimeoutSec: 10, Content: "alpha 0000 i"}); ret.Error != nil || ret.Output != "0 " {
		t.Fatal(ret)
	}
	// Decrypt file using good key
	if ret := decrypt.Execute(Command{TimeoutSec: 10, Content: "alpha 44a4 a"}); ret.Error != nil || ret.Output != "1 abc" {
		t.Fatal(ret)
	}
}
