package inet

import (
	"strings"
	"sync"
)

const HTTPPublicIPTimeoutSec = 3 // HTTPPublicIPTimeoutSec is the HTTP timeout for determining public IP address.

var (
	// isGCE is true only if IsGCE function has determined that the program is running on Google compute engine.
	isGCE     bool
	isGCEOnce = new(sync.Once)
)

// IsGCE returns true only if the program is running on Google compute engine (or Google cloud platform, same thing).
func IsGCE() bool {
	isGCEOnce.Do(func() {
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
			Header:     map[string][]string{"Metadata-Flavor": {"Google"}},
		}, "http://169.254.169.254/computeMetadata/v1/project/project-id")
		if err == nil && resp.StatusCode/200 == 1 {
			isGCE = true
		}
	})
	return isGCE
}

/*
GetPublicIP returns the latest public IP address of the computer. If the IP address cannot be determined, it will return
an empty string. The function may take up to 3 seconds to return a value.
*/
func GetPublicIP() string {
	// There are four ways to retrieve IP address and one failure scenario that returns empty string
	ipRetrieval := new(sync.WaitGroup)
	ipRetrieval.Add(4)
	ipResult := make(chan string, 5)
	// GCE internal
	go func() {
		defer ipRetrieval.Done()
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
			Header:     map[string][]string{"Metadata-Flavor": {"Google"}},
		}, "http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip")
		if err == nil && resp.StatusCode/200 == 1 {
			ipResult <- strings.TrimSpace(string(resp.Body))
		}
	}()
	// AWS internal
	go func() {
		defer ipRetrieval.Done()
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
		}, "http://169.254.169.254/2016-09-02/meta-data/public-ipv4")
		if err == nil && resp.StatusCode/200 == 1 {
			ipResult <- strings.TrimSpace(string(resp.Body))
		}
	}()
	// AWS public
	go func() {
		defer ipRetrieval.Done()
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
		}, "http://checkip.amazonaws.com")
		if err == nil && resp.StatusCode/200 == 1 {
			ipResult <- strings.TrimSpace(string(resp.Body))
		}
	}()
	// IPFY public
	go func() {
		defer ipRetrieval.Done()
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
		}, "https://api.ipify.org")
		if err == nil && resp.StatusCode/200 == 1 {
			ipResult <- strings.TrimSpace(string(resp.Body))
		}
	}()
	// After all four ways failed to determine public IP, return an empty string.
	go func() {
		ipRetrieval.Wait()
		ipResult <- ""
	}()
	return <-ipResult
}
