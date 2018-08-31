package inet

import (
	"strings"
	"sync"
	"time"
)

const HTTPPublicIPTimeoutSec = 10 // HTTPPublicIPTimeoutSec is the HTTP timeout for determining public IP address.

var (
	// isGCE is true only if IsGCE function has determined that the program is running on Google compute engine.
	isGCE     bool
	isGCEOnce = new(sync.Once)

	// isAzure is true only if IsAzure function has determined that the program is running on Microsoft Azure.
	isAzure     bool
	isAzureOnce = new(sync.Once)
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

// IsAzure returns true only if the program is running on Microsoft Azure virtual machine.
func IsAzure() bool {
	isAzureOnce.Do(func() {
		/*
			According to https://docs.microsoft.com/en-us/azure/virtual-machines/windows/instance-metadata-service
			As of 2018-08-31, the metadata API version "2017-08-01" is the only version supported across all regions,
			including Government, China, and Germany.
		*/
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
			Header:     map[string][]string{"Metadata": {"true"}},
		}, "http://169.254.169.254/metadata/instance?api-version=2017-08-01")
		if err == nil && resp.StatusCode/200 == 1 && strings.Contains(string(resp.Body), "subscriptionId") {
			isAzure = true
		}
	})
	return isAzure
}

/*
GetPublicIP returns the latest public IP address of the computer. If the IP address cannot be determined, it will return
an empty string. The function may take up to 10 seconds to return to caller.
*/
func GetPublicIP() string {
	/*
		Kick off multiple routines to determine public IP at the same time. Each routine uses a different approach, the
		fastest valid response will be returned to caller. Usually the public cloud metadata endpoints are the fastest
		and slightly more trustworthy.
	*/

	// GCE internal
	gceInternal := make(chan string, 1)
	go func() {
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
			Header:     map[string][]string{"Metadata-Flavor": {"Google"}},
		}, "http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip")
		if err == nil && resp.StatusCode/200 == 1 {
			if respBody := strings.TrimSpace(string(resp.Body)); respBody != "" {
				gceInternal <- respBody
			}
		}
	}()
	// AWS internal
	awsInternal := make(chan string, 1)
	go func() {
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
		}, "http://169.254.169.254/2018-03-28/meta-data/public-ipv4")
		if err == nil && resp.StatusCode/200 == 1 {
			if respBody := strings.TrimSpace(string(resp.Body)); respBody != "" {
				awsInternal <- respBody
			}
		}
	}()
	// Azure internal
	azureInternal := make(chan string, 1)
	go func() {
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
			Header:     map[string][]string{"Metadata": {"true"}},
		}, "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2017-12-01&format=text")
		if err == nil && resp.StatusCode/200 == 1 {
			if respBody := strings.TrimSpace(string(resp.Body)); respBody != "" {
				azureInternal <- respBody
			}
		}
	}()
	// AWS public
	awsPublic := make(chan string, 1)
	go func() {
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
		}, "http://checkip.amazonaws.com")
		if err == nil && resp.StatusCode/200 == 1 {
			if respBody := strings.TrimSpace(string(resp.Body)); respBody != "" {
				awsPublic <- respBody
			}
		}
	}()
	// ipfy.org
	ipfyPublic := make(chan string, 1)
	go func() {
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
		}, "https://api.ipify.org")
		if err == nil && resp.StatusCode/200 == 1 {
			if respBody := strings.TrimSpace(string(resp.Body)); respBody != "" {
				ipfyPublic <- respBody
			}
		}
	}()
	select {
	case ip := <-gceInternal:
		return ip
	case ip := <-awsInternal:
		return ip
	case ip := <-azureInternal:
		return ip
	case ip := <-awsPublic:
		return ip
	case ip := <-ipfyPublic:
		return ip
	case <-time.After(HTTPPublicIPTimeoutSec * time.Second):
		return ""
	}
}
