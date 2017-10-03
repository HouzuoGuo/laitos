package inet

import "testing"

func TestDoHTTP(t *testing.T) {
	// I hope nobody's buying that domain name simply to mess with this test case ^_______^
	resp, err := DoHTTP(Request{
		TimeoutSec: 30,
	}, "https://a-very-bad-domain-name-nonnnnnnbreeiunsdvc.rich")
	if err == nil {
		t.Fatal("did not error")
	}
	if resp.Non2xxToError() == nil {
		t.Fatal("did not error")
	}

	resp, err = DoHTTP(Request{
		TimeoutSec: 30,
	}, "https://github.com")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Non2xxToError() != nil {
		t.Fatal(err)
	}
}
