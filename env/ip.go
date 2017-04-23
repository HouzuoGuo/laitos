package env

import (
	"github.com/HouzuoGuo/laitos/httpclient"
	"strings"
	"sync"
)

const HTTPPublicIPTimeout = 10 // Timeout for outgoing HTTP

// Return my public IP address, or empty if public IP cannot be determined. The function will take several seconds to run!
func GetPublicIP() string {
	ipChan := make(chan string, 4) // IP addresses from three indepent ways + 1 failure send
	wg := new(sync.WaitGroup)
	wg.Add(3) // three independent ways to retrieve IP
	// GCE method
	go func() {
		defer wg.Done()
		resp, err := httpclient.DoHTTP(httpclient.Request{
			TimeoutSec: HTTPPublicIPTimeout,
			Header:     map[string][]string{"Metadata-Flavor": []string{"Google"}},
		}, "http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip")
		if err == nil && resp.StatusCode/200 == 1 {
			ipChan <- strings.TrimSpace(string(resp.Body))
		}
	}()
	// AWS method
	go func() {
		defer wg.Done()
		resp, err := httpclient.DoHTTP(httpclient.Request{
			TimeoutSec: HTTPPublicIPTimeout,
		}, "http://169.254.169.254/2016-09-02/meta-data/public-ipv4")
		if err == nil && resp.StatusCode/200 == 1 {
			ipChan <- strings.TrimSpace(string(resp.Body))
		}
	}()
	// IPFY method
	go func() {
		defer wg.Done()
		resp, err := httpclient.DoHTTP(httpclient.Request{
			TimeoutSec: HTTPPublicIPTimeout,
		}, "https://api.ipify.org")
		if err == nil && resp.StatusCode/200 == 1 {
			ipChan <- strings.TrimSpace(string(resp.Body))
		}
	}()
	// Failure mode - all three independent ways fail to retrieve IP
	go func() {
		wg.Wait()
		ipChan <- ""
	}()
	return <-ipChan
}
