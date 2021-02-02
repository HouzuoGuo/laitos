package handler

import (
	"net/http"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HandlePrometheus serves metrics readings collected by prometheus' global registry and its "default" gatherer.
type HandlePrometheus struct {
	metricHandler http.Handler
}

// Initialise initialises the prometheus HTTP handler only if prometheus integration has been enabled globally.
// Initialising the handler while prometheus integration is not enabled will not result in an error, and the handler
// will simply respond with HTTP status Service Unavailable to the clients.
func (prom *HandlePrometheus) Initialise(logger lalog.Logger, _ *toolbox.CommandProcessor, stripURLPrefixFromResponse string) error {
	if misc.EnablePrometheusIntegration {
		prom.metricHandler = promhttp.InstrumentMetricHandler(
			prometheus.DefaultRegisterer, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}),
		)
	}
	return nil
}

// Handle responds to the client request with prometheus metrics information in plain text.
func (prom *HandlePrometheus) Handle(w http.ResponseWriter, r *http.Request) {
	if prom.metricHandler == nil {
		http.Error(w, "prometheus integration is not enabled (-prominteg=true)", http.StatusServiceUnavailable)
		return
	}
	prom.metricHandler.ServeHTTP(w, r)
}

// GetRateLimitFactor returns the rate limit factor of this HTTP handler type.
func (prom *HandlePrometheus) GetRateLimitFactor() int {
	return 1
}

// SelfTest always returns nil as no self test capability is provided to prometheus.
func (prom *HandlePrometheus) SelfTest() error {
	return nil
}
