package env

import (
	"github.com/HouzuoGuo/laitos/httpclient"
	"strings"
	"sync"
)

const HTTPPublicIPTimeoutSec = 5 // HTTPPublicIPTimeoutSec is the timeout of outgoing HTTP requests for retrieval of public IP address.

/*
GetPublicIP returns the latest public IP address of the computer. If the IP address cannot be retrieved, it returns empty
string.
The function may take up to 5 seconds to return a value.
*/
func GetPublicIP() string {
	ipChan := make(chan string, 4) // IP addresses from three indepent ways + 1 failure send
	wg := new(sync.WaitGroup)
	wg.Add(3) // three independent ways to retrieve IP
	// GCE method
	go func() {
		defer wg.Done()
		resp, err := httpclient.DoHTTP(httpclient.Request{
			TimeoutSec: HTTPPublicIPTimeoutSec,
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
			TimeoutSec: HTTPPublicIPTimeoutSec,
		}, "http://169.254.169.254/2016-09-02/meta-data/public-ipv4")
		if err == nil && resp.StatusCode/200 == 1 {
			ipChan <- strings.TrimSpace(string(resp.Body))
		}
	}()
	// IPFY method
	go func() {
		defer wg.Done()
		resp, err := httpclient.DoHTTP(httpclient.Request{
			TimeoutSec: HTTPPublicIPTimeoutSec,
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
