package env

import (
	"github.com/HouzuoGuo/websh/httpclient"
	"strings"
	"sync"
)

const HTTPPublicIPTimeout = 10 // Timeout for outgoing HTTP

// Return my public IP address, or empty if public IP cannot be determined. The function will take several seconds to run!
func GetPublicIP() string {
	var awsIP, gceIP, ipifyIP string
	wg := new(sync.WaitGroup)
	wg.Add(3)
	// GCE method
	go func() {
		defer wg.Done()
		resp, err := httpclient.DoHTTP(httpclient.Request{
			TimeoutSec: HTTPPublicIPTimeout,
			Header:     map[string][]string{"Metadata-Flavor": []string{"Google"}},
		}, "http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip")
		if err == nil && resp.StatusCode/200 == 1 {
			gceIP = strings.TrimSpace(string(resp.Body))
		}
	}()
	// AWS method
	go func() {
		defer wg.Done()
		resp, err := httpclient.DoHTTP(httpclient.Request{
			TimeoutSec: HTTPPublicIPTimeout,
		}, "http://169.254.169.254/2016-09-02/meta-data/public-ipv4")
		if err == nil && resp.StatusCode/200 == 1 {
			awsIP = strings.TrimSpace(string(resp.Body))
		}
	}()
	// IPFY method
	go func() {
		defer wg.Done()
		resp, err := httpclient.DoHTTP(httpclient.Request{
			TimeoutSec: HTTPPublicIPTimeout,
		}, "https://api.ipify.org")
		if err == nil && resp.StatusCode/200 == 1 {
			ipifyIP = strings.TrimSpace(string(resp.Body))
		}
	}()
	wg.Wait()
	if awsIP != "" {
		return awsIP
	} else if gceIP != "" {
		return gceIP
	} else {
		return ipifyIP
	}
}
