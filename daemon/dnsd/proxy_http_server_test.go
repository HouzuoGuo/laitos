package dnsd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
)

func testProxyServer(t *testing.T, httpProxyServer *HTTPProxyServer) {
	// HTTP proxy request -> HTTP proxy server -> local TC -> DNS client -> remote TC -> DNS server.
	t.Run("http proxy request", func(t *testing.T) {
		proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%d", httpProxyServer.Address, httpProxyServer.Port))
		if err != nil {
			t.Fatal(err)
		}
		proxyClient := &http.Client{Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}}
		resp, err := proxyClient.Get("http://neverssl.com")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode/200 != 1 {
			t.Fatal("unexpected http response status code", resp.StatusCode)
		}
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("unexpected http response body error: %v", err)
		}
		if !strings.Contains(strings.ToLower(string(respBody)), "</html>") {
			t.Fatalf("unexpected http resposne body: %v", string(respBody))
		}
	})

	t.Run("https proxy request", func(t *testing.T) {
		proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%d", httpProxyServer.Address, httpProxyServer.Port))
		if err != nil {
			t.Fatal(err)
		}
		proxyClient := &http.Client{Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}}
		resp, err := proxyClient.Get("https://captive.apple.com/")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode/200 != 1 {
			t.Fatal("unexpected https response status code", resp.StatusCode)
		}
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("unexpected https response body error: %v", err)
		}
		if !strings.Contains(strings.ToLower(string(respBody)), "</html>") {
			t.Fatalf("unexpected https response body: %v", string(respBody))
		}
	})

	t.Run("https proxy with heavy client losses", func(t *testing.T) {
		httpProxyServer.dropPercentage = 5
		proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%d", httpProxyServer.Address, httpProxyServer.Port))
		if err != nil {
			t.Fatal(err)
		}
		proxyClient := &http.Client{Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}}
		resp, err := proxyClient.Get("https://captive.apple.com/")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode/200 != 1 {
			t.Fatal("unexpected https response status code", resp.StatusCode)
		}
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("unexpected https response body error: %v", err)
		}
		if !strings.Contains(strings.ToLower(string(respBody)), "</html>") {
			t.Fatalf("unexpected https response body: %v", string(respBody))
		}
	})
}

func TestHTTPProxyServer_CNAME(t *testing.T) {
	// Start a DNS server with the TCP-over-DNS proxy built-in.
	dnsProxyServer := &Daemon{
		Address:             "127.0.0.1",
		AllowQueryFromCidrs: []string{"127.0.0.0/8"},
		PerIPLimit:          999,
		MyDomainNames:       []string{"example.test"},
		UDPPort:             61844,
		TCPPort:             12880,
		TCPProxy: &Proxy{
			RequestOTPSecret: "testtest",
		},
	}
	if err := dnsProxyServer.Initialise(); err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := dnsProxyServer.StartAndBlock(); err != nil {
			panic(err)
		}
	}()
	if !misc.ProbePort(30*time.Second, dnsProxyServer.Address, dnsProxyServer.TCPPort) {
		t.Fatal("DNS proxy server did not start on time")
	}

	// Start an HTTP proxy server - tcp-over-DNS proxy client.
	httpProxyServer := &HTTPProxyServer{
		Address:          "127.0.0.1",
		Port:             41611,
		RequestOTPSecret: dnsProxyServer.TCPProxy.RequestOTPSecret,
		Config: tcpoverdns.InitiatorConfig{
			SetConfig:               true,
			Debug:                   true,
			MaxSegmentLenExclHeader: MaxUpstreamSegmentLength(dnsProxyServer.MyDomainNames[0]),
			Timing: tcpoverdns.TimingConfig{
				ReadTimeout:               MaxProxyConnectionLifetime,
				WriteTimeout:              MaxProxyConnectionLifetime,
				RetransmissionInterval:    7 * time.Second,
				SlidingWindowWaitDuration: 3000 * time.Millisecond,
				KeepAliveInterval:         1500 * time.Millisecond,
				AckDelay:                  500 * time.Millisecond,
			},
		},
		Debug:       true,
		DNSResolver: fmt.Sprintf("%s:%d", dnsProxyServer.Address, dnsProxyServer.UDPPort),
		DNSHostName: dnsProxyServer.MyDomainNames[0],
	}
	if err := httpProxyServer.Initialise(context.Background()); err != nil {
		t.Fatal(err)
	}
	daemonStopped := make(chan struct{}, 1)
	go func() {
		if err := httpProxyServer.StartAndBlock(); err != nil {
			panic(err)
		}
		daemonStopped <- struct{}{}
	}()
	if !misc.ProbePort(30*time.Second, httpProxyServer.Address, httpProxyServer.Port) {
		t.Fatal("HTTP proxy server did not start on time")
	}
	testProxyServer(t, httpProxyServer)

	httpProxyServer.Stop()
	<-daemonStopped
	// Repeatedly stopping the daemon should have no negative consequences
	httpProxyServer.Stop()
	httpProxyServer.Stop()
}

func TestHTTPProxyServer_TXT(t *testing.T) {
	// Start a DNS server with the TCP-over-DNS proxy built-in.
	dnsProxyServer := &Daemon{
		Address:             "127.0.0.1",
		AllowQueryFromCidrs: []string{"127.0.0.0/8"},
		PerIPLimit:          999,
		MyDomainNames:       []string{"example.test"},
		UDPPort:             45278,
		TCPPort:             32148,
		TCPProxy: &Proxy{
			RequestOTPSecret: "testtest",
		},
	}
	if err := dnsProxyServer.Initialise(); err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := dnsProxyServer.StartAndBlock(); err != nil {
			panic(err)
		}
	}()
	if !misc.ProbePort(30*time.Second, dnsProxyServer.Address, dnsProxyServer.TCPPort) {
		t.Fatal("DNS proxy server did not start on time")
	}

	// Start an HTTP proxy server - tcp-over-DNS proxy client.
	httpProxyServer := &HTTPProxyServer{
		Address:                 "127.0.0.1",
		Port:                    61122,
		RequestOTPSecret:        dnsProxyServer.TCPProxy.RequestOTPSecret,
		DownstreamSegmentLength: MaxDownstreamSegmentLengthTXT(dnsProxyServer.MyDomainNames[0]),
		Config: tcpoverdns.InitiatorConfig{
			SetConfig:               true,
			Debug:                   true,
			MaxSegmentLenExclHeader: MaxUpstreamSegmentLength(dnsProxyServer.MyDomainNames[0]),
			Timing: tcpoverdns.TimingConfig{
				ReadTimeout:               MaxProxyConnectionLifetime,
				WriteTimeout:              MaxProxyConnectionLifetime,
				RetransmissionInterval:    7 * time.Second,
				SlidingWindowWaitDuration: 3000 * time.Millisecond,
				KeepAliveInterval:         1500 * time.Millisecond,
				AckDelay:                  500 * time.Millisecond,
			},
		},
		Debug:             true,
		EnableTXTRequests: true,
		DNSResolver:       fmt.Sprintf("%s:%d", dnsProxyServer.Address, dnsProxyServer.UDPPort),
		DNSHostName:       dnsProxyServer.MyDomainNames[0],
	}
	if err := httpProxyServer.Initialise(context.Background()); err != nil {
		t.Fatal(err)
	}
	daemonStopped := make(chan struct{}, 1)
	go func() {
		if err := httpProxyServer.StartAndBlock(); err != nil {
			panic(err)
		}
		daemonStopped <- struct{}{}
	}()
	if !misc.ProbePort(30*time.Second, httpProxyServer.Address, httpProxyServer.Port) {
		t.Fatal("HTTP proxy server did not start on time")
	}
	testProxyServer(t, httpProxyServer)

	httpProxyServer.Stop()
	<-daemonStopped
	// Repeatedly stopping the daemon should have no negative consequences
	httpProxyServer.Stop()
	httpProxyServer.Stop()
}
