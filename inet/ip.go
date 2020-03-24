package inet

import (
	"strings"
	"sync"
	"time"
)

// HTTPPublicIPTimeoutSec is the timeout in seconds used when determining public IP and cloud detection.
const HTTPPublicIPTimeoutSec = 10

var (
	// isAWS is true only if IsAWS function has determined that the program is running on Amazon Web Service.
	isAWS     bool
	isAWSOnce = new(sync.Once)

	// isGCE is true only if IsGCE function has determined that the program is running on Google compute engine.
	isGCE     bool
	isGCEOnce = new(sync.Once)

	// isAzure is true only if IsAzure function has determined that the program is running on Microsoft Azure.
	isAzure     bool
	isAzureOnce = new(sync.Once)

	// isAlibaba is true only if IsAlibaba has determined that the program is running on Alibaba Cloud.
	isAlibaba     bool
	isAlibabaOnce = new(sync.Once)

	// lastIP is the latest public IP retrieved recently.
	lastPublicIP string
	//lastIPMutex protects last public IP from concurrent modifcation.
	lastPublicIPMutex = new(sync.Mutex)
	// lastIPTimeStamp is the time at which the last public IP was determined.
	lastPublicIPTimeStamp time.Time
)

// IsAWS returns true only if the program is running on Amazon Web Service.
func IsAWS() bool {
	isAWSOnce.Do(func() {
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
			MaxBytes:   64,
		}, "http://169.254.169.254/2018-09-24/meta-data/ami-id")
		if err == nil && resp.StatusCode/200 == 1 {
			isAWS = true
		}
	})
	return isAWS
}

// IsGCE returns true only if the program is running on Google compute engine (or Google cloud platform, same thing).
func IsGCE() bool {
	isGCEOnce.Do(func() {
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
			MaxBytes:   64,
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
			As of 2019-05-23, the metadata API version "2018-10-01" is the only version supported across all regions,
			including Government, China, and Germany.
		*/
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
			/*
				Be aware that successful detection of Azure public cloud relies on
				the appearance of string "azure", which usually comes as:
				... "azEnvironment":"AzurePublicCloud" ...
				Therefore, ensure that the response size limit is sufficient. Normally the response does not exceed 1KB.
			*/
			MaxBytes: 4096,
			Header:   map[string][]string{"Metadata": {"true"}},
		}, "http://169.254.169.254/metadata/instance?api-version=2018-10-01")
		if err == nil && resp.StatusCode/200 == 1 && strings.Contains(strings.ToLower(string(resp.Body)), "azure") {
			isAzure = true
		}
	})
	return isAzure
}

// IsAlibaba returns true only if the program is running on Alibaba cloud.
func IsAlibaba() bool {
	isAlibabaOnce.Do(func() {
		resp, err := DoHTTP(HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
			MaxBytes:   64,
		}, "http://100.100.100.200/latest/meta-data/zone-id")
		if err == nil && resp.StatusCode/200 == 1 {
			isAlibaba = true
		}
	})
	return isAlibaba
}

/*
getPublicIP is an internal function that determines the public IP address of this computer. If the IP address cannot be
determined, it will return "0.0.0.0". It may take up to 10 seconds to return.
*/
func getPublicIP() string {
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
			MaxBytes:   64,
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
			MaxBytes:   64,
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
			MaxBytes:   64,
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
			MaxBytes:   64,
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
			MaxBytes:   64,
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
		return "0.0.0.0"
	}
}

/*
GetPublicIP returns the latest public IP address of the computer. If the IP address cannot be determined, it will return
an empty string. If the public IP has been determined recently (less than 3 minutes ago), the cached public IP will be
returned.
*/
func GetPublicIP() string {
	/*
		Normally it is quite harmless to retrieve public IP address in short succession, however the cloud internal endpoints
		are not reachable on an ordinary computer, and by attempting to connect to them (e.g. 169.254.169.254), there is going
		to be a half-open TCP connection that will stick around for a while before OS cleans it up. When such half-open
		connections pile up, the OS will have exhausted local port numbers and refuse to make more outgoing connections.
	*/
	lastPublicIPMutex.Lock()
	defer lastPublicIPMutex.Unlock()
	if time.Now().Unix()-lastPublicIPTimeStamp.Unix() > 3*60 {
		lastPublicIP = getPublicIP()
		lastPublicIPTimeStamp = time.Now()
	}
	return lastPublicIP
}
