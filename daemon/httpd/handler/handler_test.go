package handler

import (
	"testing"
)

func TestXMLEscape(t *testing.T) {
	if out := XMLEscape("<!--&ha"); out != "&lt;!--&amp;ha" {
		t.Fatal(out)
	}
}

// API handler tests are written in httpd.go and run in httpd_test.go
