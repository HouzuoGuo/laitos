package inet

import (
	"strings"
	"sync"
	"time"
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
	// Use four different ways to retrieve IP address
	// GCE internal
	gceInternal := make(chan string)
	go func() {
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
			Header:     map[string][]string{"Metadata-Flavor": {"Google"}},
		}, "http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip")
		if err == nil && resp.StatusCode/200 == 1 {
			gceInternal <- strings.TrimSpace(string(resp.Body))
		}
	}()
	// AWS internal
	awsInternal := make(chan string)
	go func() {
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
		}, "http://169.254.169.254/2016-09-02/meta-data/public-ipv4")
		if err == nil && resp.StatusCode/200 == 1 {
			awsInternal <- strings.TrimSpace(string(resp.Body))
		}
	}()
	// AWS public
	awsPublic := make(chan string)
	go func() {
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
		}, "http://checkip.amazonaws.com")
		if err == nil && resp.StatusCode/200 == 1 {
			awsPublic <- strings.TrimSpace(string(resp.Body))
		}
	}()
	// ipfy.org
	ipfyPublic := make(chan string)
	go func() {
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
		}, "https://api.ipify.org")
		if err == nil && resp.StatusCode/200 == 1 {
			ipfyPublic <- strings.TrimSpace(string(resp.Body))
		}
	}()
	select {
	case ip := <-gceInternal:
		return ip
	case ip := <-awsInternal:
		return ip
	case ip := <-awsPublic:
		return ip
	case ip := <-ipfyPublic:
		return ip
	case <-time.After(HTTPPublicIPTimeoutSec * time.Second):
		return ""
	}
}
