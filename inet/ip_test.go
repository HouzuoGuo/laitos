package inet

import (
	"testing"
)

func TestGetPublicIP(t *testing.T) {
	if ip := GetPublicIP(); len(ip) < 7 || len(ip) > 46 {
		t.Fatal(ip)
	}
}

func TestCloudDetection(t *testing.T) {
	// Just make sure they do not crash
	IsAzure()
	IsAlibaba()
	IsGCE()
}
