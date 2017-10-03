package inet

import (
	"os"
	"testing"
)

func TestGetPublicIP(t *testing.T) {
	if os.Getenv("TRAVIS") != "" {
		t.Skip("This test cannot pass on Travis")
	}
	if ip := GetPublicIP(); len(ip) < 7 || len(ip) > 15 {
		t.Fatal(ip)
	}
}
