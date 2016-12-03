package main

import (
	"testing"
)

func TestFacebook(t *testing.T) {
	t.Skip()
	fb := FacebookClient{
		AccessToken: "FILLME",
	}
	if err := fb.WriteStatus(10, "hi there test123"); err != nil {
		t.Fatal(err)
	}
}
