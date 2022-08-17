package inet

import (
	"testing"
)

func TestGetPublicIP(t *testing.T) {
	ip := GetPublicIP()
	if len(ip) < 7 || len(ip) > 46 {
		t.Fatal(ip)
	}
	if ip2 := GetPublicIP(); ip2.String() != ip.String() {
		t.Fatal(ip, ip2)
	}
}

func TestCloudDetection(t *testing.T) {
	// Just make sure they do not crash
	IsAWS()
	IsAzure()
	IsAlibaba()
	IsGCE()
}
