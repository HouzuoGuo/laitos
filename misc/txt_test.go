package misc

import (
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestEditKeyValue(t *testing.T) {
	tmp, err := os.CreateTemp("", "laitos-TestEditKeyValue")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	sampleContent := `
# blah blah
#
[Resolve]
#DNS=
#FallbackDNS=
#Domains=
#LLMNR=yes
#MulticastDNS=yes
#DNSSEC=no
#Cache=yes
  TestKey = TestValue
DNSStubListener=udp`

	if err := os.WriteFile(tmp.Name(), []byte(sampleContent), 0600); err != nil {
		t.Fatal(err)
	}
	// Set DNSStubListener=no and verify
	if err := EditKeyValue(tmp.Name(), "DNSStubListener", "no"); err != nil {
		t.Fatal(err)
	}
	matchContent := `
# blah blah
#
[Resolve]
#DNS=
#FallbackDNS=
#Domains=
#LLMNR=yes
#MulticastDNS=yes
#DNSSEC=no
#Cache=yes
  TestKey = TestValue
DNSStubListener=no`
	if content, err := os.ReadFile(tmp.Name()); err != nil || string(content) != matchContent {
		t.Fatal(err, string(content), "\n", content, "\n", []byte(matchContent))
	}
	// Set TestKey = NewValue and verify
	if err := EditKeyValue(tmp.Name(), "TestKey", "NewValue"); err != nil {
		t.Fatal(err)
	}
	matchContent = `
# blah blah
#
[Resolve]
#DNS=
#FallbackDNS=
#Domains=
#LLMNR=yes
#MulticastDNS=yes
#DNSSEC=no
#Cache=yes
TestKey=NewValue
DNSStubListener=no`
	if content, err := os.ReadFile(tmp.Name()); err != nil || string(content) != matchContent {
		t.Fatal(err, string(content))
	}
	// Set NewKey=NewValue and verify
	if err := EditKeyValue(tmp.Name(), "NewKey", "NewValue"); err != nil {
		t.Fatal(err)
	}
	matchContent = `
# blah blah
#
[Resolve]
#DNS=
#FallbackDNS=
#Domains=
#LLMNR=yes
#MulticastDNS=yes
#DNSSEC=no
#Cache=yes
TestKey=NewValue
DNSStubListener=no
NewKey=NewValue`
	if content, err := os.ReadFile(tmp.Name()); err != nil || string(content) != matchContent {
		t.Fatal(err, string(content))
	}
}

func TestReadAllUpTo(t *testing.T) {
	if b, err := ReadAllUpTo(nil, 10); err != ErrInputReaderNil || !reflect.DeepEqual(b, []byte{}) {
		t.Fatal(b, err)
	}
	if b, err := ReadAllUpTo(strings.NewReader(""), -1); err != ErrInputCapacityInvalid || !reflect.DeepEqual(b, []byte{}) {
		t.Fatal(b, err)
	}
	if b, err := ReadAllUpTo(strings.NewReader("a"), 0); err != nil || !reflect.DeepEqual(b, []byte{}) {
		t.Fatal(b, err)
	}
	if b, err := ReadAllUpTo(strings.NewReader("a"), 1); err != nil || !reflect.DeepEqual(b, []byte{'a'}) {
		t.Fatal(b, err)
	}
	if b, err := ReadAllUpTo(strings.NewReader("a"), 20000); err != nil || !reflect.DeepEqual(b, []byte{'a'}) {
		t.Fatal(b, err)
	}

	r := strings.NewReader(strings.Repeat("a", 20000))
	if b, err := ReadAllUpTo(r, 2); err != nil || !reflect.DeepEqual(b, []byte{'a', 'a'}) {
		t.Fatal(b, err)
	}
	if i, err := r.Seek(0, io.SeekCurrent); i != 2 {
		t.Fatal(i, err)
	}
	// Read remainder of the input
	if b, err := ReadAllUpTo(r, 20000); err != nil || len(b) != 20000-2 {
		t.Fatal(b, err)
	}
	if b, err := ReadAllUpTo(r, 20000); err != nil || !reflect.DeepEqual(b, []byte{}) {
		t.Fatal(b, err)
	}
}

func TestEncryptDecrypt(t *testing.T) {
	tmp, err := os.CreateTemp("", "laitos-TestEncryptDecrypt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	sampleContent := `01234567890abcdefghijklmnopqrstuvwxyz`
	if err := os.WriteFile(tmp.Name(), []byte(sampleContent), 0600); err != nil {
		t.Fatal(err)
	}

	if content, encrypted, err := IsEncrypted(tmp.Name()); err != nil || encrypted || string(content) != sampleContent {
		t.Fatal(err, encrypted, string(content))
	}

	if contents, isEncrypted, err := DecryptIfNecessary("this is a key", tmp.Name()); err != nil || len(isEncrypted) != 1 || isEncrypted[0] ||
		len(contents) != 1 || string(contents[0]) != sampleContent {
		t.Fatal(err, isEncrypted, contents)
	}

	if err := Encrypt(tmp.Name(), []byte("this is a key")); err != nil {
		t.Fatal(err)
	}
	if content, encrypted, err := IsEncrypted(tmp.Name()); err != nil || !encrypted || len(content) < len(sampleContent) {
		t.Fatal(err, encrypted)
	}
	if encryptedContent, err := os.ReadFile(tmp.Name()); err != nil || strings.Contains(string(encryptedContent), "123") {
		t.Fatal(err, string(encryptedContent))
	}

	if content, err := Decrypt(tmp.Name(), "this is a key"); err != nil || string(content) != sampleContent {
		t.Fatal(err, string(content))
	}

	// Decrypt with wrong key should not yield any useful content
	if content, err := Decrypt(tmp.Name(), "wrong key"); err != nil || strings.Contains(string(content), "123") {
		t.Fatal(err, string(content))
	}

	if contents, isEncrypted, err := DecryptIfNecessary("this is a key", tmp.Name()); err != nil || len(isEncrypted) != 1 || !isEncrypted[0] ||
		len(contents) != 1 || string(contents[0]) != sampleContent {
		t.Fatal(err, isEncrypted, contents)
	}
}
