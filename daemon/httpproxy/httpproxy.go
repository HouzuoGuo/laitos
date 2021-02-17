package httpproxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/httpd/middleware"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// DefaultPort is the port number the daemon will listen on in the absence of port number specified by user.
	DefaultPort = 210
	// TestPort is used exclusively by test cases to configure daemon under testing.
	TestPort = 54112
	// IOTimeout is the maximum duration of an entire proxy request. The number took inspiration from the default settings in Squid software.
	IOTimeout = time.Duration(10 * time.Minute)
	// MaxRequestBodyBytes is the maximum size accepted for the entire request body of an HTTP proxy request.
	// The size does not apply to HTTPS proxy request (HTTP CONNECT).
	MaxRequestBodyBytes = 2 * 1024 * 1024
)

// Daemon offers an HTTP proxy capable of handling both HTTP and HTTPS destinations.
type Daemon struct {
	// Address is the IP address to listen on, e.g. 0.0.0.0 to listen on all network interfaces.
	Address string `json:"Address"`
	// Port to listen on.
	Port int `json:"Port"`
	// PerIPLimit is the approximate number of requests a client (identified by its IP) can make in a second.
	PerIPLimit int `json:"PerIPLimit"`
	// AllowFromCidrs is a list of CIDRs that client address must reside in to be eligible to use this HTTP proxy daemon.
	AllowFromCidrs []string `json:"AllowFromCidrs"`
	// Processor is a toolbox command processor that collects client subject reports for its store&forward message processor app.
	// Though the HTTP proxy daemon itself is incapable of executing app commands, the daemon will however subjects of the message
	// processor (computers) to use the proxy daemon. This saves the effort of having to figure out users' Internet CIDR block
	// and placing them into AllowFromCidrs.
	// This mechanism exists in the DNS daemon in a similar form.
	CommandProcessor *toolbox.CommandProcessor `json:"-"`

	allowFromIPNets []*net.IPNet
	proxyHandler    http.HandlerFunc
	rateLimit       *misc.RateLimit
	logger          lalog.Logger
	httpServer      *http.Server
}

// Initialise validates configuration parameters and initialises the internal state of the daemon.
func (daemon *Daemon) Initialise() error {
	if daemon.Address == "" {
		daemon.Address = "0.0.0.0"
	}
	if daemon.PerIPLimit < 1 {
		daemon.PerIPLimit = 100
	}
	if daemon.Port == 0 {
		daemon.Port = DefaultPort
	}
	if daemon.CommandProcessor == nil {
		daemon.CommandProcessor = toolbox.GetEmptyCommandProcessor()
	}
	daemon.logger = lalog.Logger{ComponentName: "httpproxy", ComponentID: []lalog.LoggerIDField{{Key: "Port", Value: strconv.Itoa(daemon.Port)}}}
	daemon.rateLimit = &misc.RateLimit{
		UnitSecs: 1,
		MaxCount: daemon.PerIPLimit,
		Logger:   daemon.logger,
	}
	daemon.rateLimit.Initialise()
	// Parse allowed CIDRs into IP nets
	daemon.allowFromIPNets = make([]*net.IPNet, 0)
	for _, cidrStr := range daemon.AllowFromCidrs {
		_, cidrNet, err := net.ParseCIDR(cidrStr)
		if err != nil {
			return fmt.Errorf("httpproxy.Initialise: failed to parse string \"%s\" for a CIDR that is allowed to use this proxy server", cidrStr)
		}
		daemon.allowFromIPNets = append(daemon.allowFromIPNets, cidrNet)
	}
	// Collect proxy request and response stats in prometheus histograms
	var handlerDurationHistogram, responseTimeToFirstByteHistogram, responseSizeHistogram *prometheus.HistogramVec
	if misc.EnablePrometheusIntegration {
		metricsLabelNames := []string{middleware.PrometheusHandlerTypeLabel, middleware.PrometheusHandlerLocationLabel, middleware.PrometheusHandlerHostLabel}
		handlerDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "laitos_httpproxy_handler_duration_seconds",
			Help:    "The run-duration of HTTP proxy responses in seconds",
			Buckets: []float64{0.025, 0.050, 0.1, 0.1375, 0.2125, 0.25, 0.375, 0.4375, 0.5, 0.75, 0.875, 1.25, 1.5, 2, 3, 5, 8, 12, 20},
		}, metricsLabelNames)
		responseTimeToFirstByteHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "laitos_httpproxy_response_time_to_first_byte_seconds",
			Help:    "The time-to-first-byte of HTTP proxy responses in seconds",
			Buckets: []float64{0.025, 0.050, 0.1, 0.1375, 0.2125, 0.25, 0.375, 0.4375, 0.5, 0.75, 0.875, 1.25, 1.5, 2, 3, 5, 8, 12, 20},
		}, metricsLabelNames)
		responseSizeHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "laitos_httpproxy_response_size_bytes",
			Help:    "The size of response produced by HTTP proxy responses function in bytes",
			Buckets: []float64{64, 256, 512, 1024, 2048, 4096, 16384, 32768, 65536, 131072, 262144, 524288, 1048576, 2097152, 4194304, 5242880, 8388608, 12582912, 16777216},
		}, metricsLabelNames)
		for _, histogram := range []*prometheus.HistogramVec{handlerDurationHistogram, responseTimeToFirstByteHistogram, responseSizeHistogram} {
			if err := prometheus.Register(histogram); err != nil {
				daemon.logger.Warning("Initialise", "", err, "failed to register prometheus metrics collectors")
			}
		}
	}
	daemon.proxyHandler = middleware.LogRequestStats(daemon.logger,
		middleware.RecordInternalStats(misc.HTTPProxyStats,
			middleware.EmergencyLockdown(
				daemon.CheckClientIPMiddleware(
					middleware.RecordPrometheusStats("httpproxy", "", handlerDurationHistogram, responseTimeToFirstByteHistogram, responseSizeHistogram,
						middleware.RateLimit(daemon.rateLimit, daemon.ProxyHandler))))))
	return nil
}

// StartAndBlock starts a web server with a specially crafted handler to serve HTTP proxy clients.
// The function will block caller until Stop is called.
func (daemon *Daemon) StartAndBlock() error {
	daemon.httpServer = &http.Server{
		Addr:         net.JoinHostPort(daemon.Address, strconv.Itoa(daemon.Port)),
		Handler:      daemon.proxyHandler,
		ReadTimeout:  IOTimeout,
		WriteTimeout: IOTimeout,
		// TODO: figure out how to handle an HTTP/2 proxy client and then reenable HTTP/2 support
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	daemon.logger.Info("StartAndBlock", "", nil, "starting now")
	if err := daemon.httpServer.ListenAndServe(); err != nil {
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		return fmt.Errorf("httpproxy.StartAndBlock.: failed to listen on %s:%d - %v", daemon.Address, daemon.Port, err)
	}
	return nil
}

// Stop the daemon.
func (daemon *Daemon) Stop() {
	if daemon.httpServer != nil {
		stopCtx, cancelFunc := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelFunc()
		if err := daemon.httpServer.Shutdown(stopCtx); err != nil {
			daemon.logger.Warning("Stop", daemon.Address, err, "failed to shutdown")
		}
	}
}

// TestHTTPProxyDaemon is used exclusively by test case to run a comprehensive test routine for the daemon's functions.
// The daemon must have already been completed with all of its configuration and successfully initialised.
// See httpproxy_test.go for the daemon initialisation routine.
func TestHTTPProxyDaemon(daemon *Daemon, t testingstub.T) {
	daemonStopped := make(chan struct{}, 1)
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Error(err)
			return
		}
		daemonStopped <- struct{}{}
	}()
	if !misc.ProbePort(1*time.Second, "127.0.0.1", daemon.Port) {
		t.Fatal("daemon did not start on time")
	}
	// Use the HTTP proxy daemon as the choice of proxy in this HTTP client
	proxyURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", daemon.Port))
	if err != nil {
		t.Fatal(err)
	}
	proxyClient := &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}}
	// Perform a regular HTTP request using the proxy server
	resp, err := proxyClient.Get("http://google.com")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode/200 != 1 {
		t.Fatal("unexpected http response status code", resp.StatusCode)
	}
	// Perform a regular HTTPS request using the proxy server
	resp, err = proxyClient.Get("https://google.com")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode/200 != 1 {
		t.Fatal("unexpected http response status code", resp.StatusCode)
	}
	daemon.Stop()
	<-daemonStopped
	// Repeatedly stopping the daemon should have no negative consequences
	daemon.Stop()
	daemon.Stop()
}
