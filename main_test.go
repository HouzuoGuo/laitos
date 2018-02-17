package main

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestReseedPseudoRandAndInBackground(t *testing.T) {
	ReseedPseudoRandAndInBackground()
}

func TestPrepareUtilitiesAndInBackground(t *testing.T) {
	PrepareUtilitiesAndInBackground()
}

func TestEditKeyValue(t *testing.T) {
	tmp, err := ioutil.TempFile("", "laitos-TestEditKeyValue")
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

	if err := ioutil.WriteFile(tmp.Name(), []byte(sampleContent), 0600); err != nil {
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
	if content, err := ioutil.ReadFile(tmp.Name()); err != nil || string(content) != matchContent {
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
	if content, err := ioutil.ReadFile(tmp.Name()); err != nil || string(content) != matchContent {
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
	if content, err := ioutil.ReadFile(tmp.Name()); err != nil || string(content) != matchContent {
		t.Fatal(err, string(content))
	}

}
