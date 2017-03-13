package env

import "testing"

func TestGetPublicIP(t *testing.T) {
	if ip := GetPublicIP(); len(ip) < 7 || len(ip) > 15 {
		t.Fatal(ip)
	}
}
