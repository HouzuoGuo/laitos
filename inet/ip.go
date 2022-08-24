package inet

import (
	"context"
	"net"
	"net/http"
	"os"
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
	lastPublicIP net.IP
	//lastIPMutex protects last public IP from concurrent modifcation.
	lastPublicIPMutex = new(sync.Mutex)
	// lastIPTimeStamp is the time at which the last public IP was determined.
	lastPublicIPTimeStamp time.Time
)

// IsAWS returns true only if the program is running on Amazon Web Service.
func IsAWS() bool {
	isAWSOnce.Do(func() {
		confirmationChan := make(chan bool, 2)
		// Maybe running on EC2 - look for EC2 metadata service
		go func() {
			resp, err := doHTTPRequestUsingClient(context.Background(), &http.Client{}, HTTPRequest{
				TimeoutSec: HTTPPublicIPTimeoutSec,
				MaxBytes:   64,
			}, "http://169.254.169.254/latest/meta-data")
			if err == nil && resp.StatusCode/200 == 1 {
				confirmationChan <- true
			}
		}()
		// Maybe running in an ECS container - look for contaienr metadata service
		go func() {
			resp, err := doHTTPRequestUsingClient(context.Background(), &http.Client{}, HTTPRequest{
				TimeoutSec: HTTPPublicIPTimeoutSec,
				MaxBytes:   64,
			}, "http://169.254.170.2/v2/metadata")
			if err == nil && resp.StatusCode/200 == 1 {
				confirmationChan <- true
			}
		}()
		select {
		case <-confirmationChan:
			isAWS = true
		case <-time.After(HTTPPublicIPTimeoutSec * time.Second):
			isAWS = false
		}
	})
	return isAWS
}

// IsGCE returns true only if the program is running on Google compute engine (or Google cloud platform, same thing).
func IsGCE() bool {
	isGCEOnce.Do(func() {
		resp, err := doHTTPRequestUsingClient(context.Background(), &http.Client{}, HTTPRequest{
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
		resp, err := doHTTPRequestUsingClient(context.Background(), &http.Client{}, HTTPRequest{
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
		resp, err := doHTTPRequestUsingClient(context.Background(), &http.Client{}, HTTPRequest{
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
func getPublicIP() net.IP {
	/*
		Kick off multiple routines to determine public IP at the same time. Each routine uses a different approach, the
		fastest valid response will be returned to caller. Usually the cloud metadata endpoints are the fastest and slightly
		more reliable.
		Avoid contacting cloud metadata endpoints unless the host is actually on public cloud. Otherwise, the connection will
		remain half open for quite a while until OS or router cleans it up.
	*/
	ip := make(chan net.IP, 5)
	// GCE internal
	go func() {
		if IsGCE() {
			resp, err := doHTTPRequestUsingClient(context.Background(), &http.Client{}, HTTPRequest{
				TimeoutSec: HTTPPublicIPTimeoutSec,
				MaxBytes:   64,
				Header:     map[string][]string{"Metadata-Flavor": {"Google"}},
			}, "http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip")
			if err == nil && resp.StatusCode/200 == 1 {
				if respBody := strings.TrimSpace(string(resp.Body)); respBody != "" {
					ip <- net.ParseIP(respBody)
				}
			}
		}
	}()
	// AWS internal
	go func() {
		if IsAWS() {
			resp, err := doHTTPRequestUsingClient(context.Background(), &http.Client{}, HTTPRequest{
				TimeoutSec: HTTPPublicIPTimeoutSec,
				MaxBytes:   64,
			}, "http://169.254.169.254/2018-03-28/meta-data/public-ipv4")
			if err == nil && resp.StatusCode/200 == 1 {
				if respBody := strings.TrimSpace(string(resp.Body)); respBody != "" {
					ip <- net.ParseIP(respBody)
				}
			}
		}
	}()
	// Azure internal
	go func() {
		if IsAzure() {
			resp, err := doHTTPRequestUsingClient(context.Background(), &http.Client{}, HTTPRequest{
				TimeoutSec: HTTPPublicIPTimeoutSec,
				MaxBytes:   64,
				Header:     map[string][]string{"Metadata": {"true"}},
			}, "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2017-12-01&format=text")
			if err == nil && resp.StatusCode/200 == 1 {
				if respBody := strings.TrimSpace(string(resp.Body)); respBody != "" {
					ip <- net.ParseIP(respBody)
				}
			}
		}
	}()
	// AWS public
	go func() {
		resp, err := doHTTPRequestUsingClient(context.Background(), &http.Client{}, HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
			MaxBytes:   64,
		}, "http://checkip.amazonaws.com")
		if err == nil && resp.StatusCode/200 == 1 {
			if respBody := strings.TrimSpace(string(resp.Body)); respBody != "" {
				ip <- net.ParseIP(respBody)
			}
		}
	}()
	// ipfy.org
	go func() {
		resp, err := doHTTPRequestUsingClient(context.Background(), &http.Client{}, HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
			MaxBytes:   64,
		}, "https://api.ipify.org")
		if err == nil && resp.StatusCode/200 == 1 {
			if respBody := strings.TrimSpace(string(resp.Body)); respBody != "" {
				ip <- net.ParseIP(respBody)
			}
		}
	}()
	select {
	case s := <-ip:
		return s
	case <-time.After(HTTPPublicIPTimeoutSec * time.Second):
		return net.IPv4(0, 0, 0, 0)
	}
}

/*
GetPublicIP returns the latest public IP address of the computer. If the IP address cannot be determined, it will return
an empty string. If the public IP has been determined recently (less than 3 minutes ago), the cached public IP will be
returned.
*/
func GetPublicIP() net.IP {
	/*
		Normally it is quite harmless to retrieve public IP address in short succession, however when laitos host's network
		fails to reach some of the IP address retrieval endpoints, such as when a home server tries to contact cloud metadata
		service on 169.254.169.254, the connection will remain half open for quite a while until the host or router cleans it up.
		Doing so in short succession (e.g. the "phonehome" daemon gets the latest public IP address several times a minute)
		quickly exhausts local port numbers, and the host OS will be incapable of making more outbound TCP connections.
		Therefore, cache the latest public IP address for up to three minutes.
	*/
	lastPublicIPMutex.Lock()
	defer lastPublicIPMutex.Unlock()
	if time.Now().Unix()-lastPublicIPTimeStamp.Unix() > 3*60 {
		newPublicIP := getPublicIP()
		if lastPublicIP.Equal(net.IPv4(0, 0, 0, 0)) || !newPublicIP.Equal(net.IPv4(0, 0, 0, 0)) {
			lastPublicIP = newPublicIP
		}
		lastPublicIPTimeStamp = time.Now()
	}
	return lastPublicIP
}

/*
GetAWSRegion returns the AWS region name specified in program environment "AWS_REGION"; if left unspecified, the function returns
region name retrieved from EC2 metadata service.
If the AWS region name cannot be determined, the function will return an empty string.
*/
func GetAWSRegion() string {
	if regionName := os.Getenv("AWS_REGION"); regionName != "" {
		return regionName
	}
	if IsAWS() {
		resp, err := doHTTPRequestUsingClient(context.Background(), &http.Client{}, HTTPRequest{
			TimeoutSec: HTTPPublicIPTimeoutSec,
			MaxBytes:   64,
		}, "http://169.254.169.254/2020-10-27/meta-data/placement/region")
		if err == nil && resp.StatusCode/200 == 1 {
			if respBody := strings.TrimSpace(string(resp.Body)); respBody != "" {
				return respBody
			}
		}
	}
	return ""
}
