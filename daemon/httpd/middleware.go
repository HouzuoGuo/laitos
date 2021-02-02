package httpd

import (
	"net/http"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/httpd/handler"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/prometheus/client_golang/prometheus"
)

// HTTPResponseRecorder is an http.ResponseWriter that helps an HTTP handler middleware to inspect the HTTP status code and size of response.
type HTTPResponseRecorder struct {
	http.ResponseWriter
	statusCode           int
	responseBodySize     int
	timestampAtWriteCall time.Time
}

// WriteHeader memorises the status code in the recorder and then invokes the underlying ResponseWriter using the same status code.
func (rec *HTTPResponseRecorder) WriteHeader(statusCode int) {
	rec.statusCode = statusCode
	rec.ResponseWriter.WriteHeader(statusCode)
}

// Write memorises the time-to-1st-byte and accumulated size of the response, and then invokes the underlying ResponseWriter using the same data buffer.
func (rec *HTTPResponseRecorder) Write(b []byte) (int, error) {
	if rec.timestampAtWriteCall.IsZero() {
		rec.timestampAtWriteCall = time.Now()
	}
	size, err := rec.ResponseWriter.Write(b)
	rec.responseBodySize += size
	return size, err
}

// DecorateWithMiddleware offers these additional features to the input handler function:
// - If emergency lock down is in effect, respond to client without invoking the actual handler function.
// - Check client IP against rate limit.
// - Limit the maximum size of HTTP request body to be processed.
// - Log and record the duration of the request.
// - Record stats in prometheus histogram.
// - Integrate with AWS x-ray.
func (daemon *Daemon) DecorateWithMiddleware(handlerTypeLabel, handlerLocationLabel string, rateLimit *misc.RateLimit, restrictedRequestSize bool, next http.HandlerFunc,
	durationHistogram, timeToFirstByteHistogram, responseSizeHistogram *prometheus.HistogramVec) http.Handler {
	promLabels := prometheus.Labels{PrometheusHandlerTypeLabel: handlerTypeLabel, PrometheusHandlerLocationLabel: handlerLocationLabel}
	var durationObs, timeToFirstByteObs, responseSizeObs prometheus.Observer
	if misc.EnablePrometheusIntegration {
		durationObs = durationHistogram.With(promLabels)
		timeToFirstByteObs = timeToFirstByteHistogram.With(promLabels)
		responseSizeObs = responseSizeHistogram.With(promLabels)
	}
	decoratedHandler := func(w http.ResponseWriter, r *http.Request) {
		// Record the duration of request handling in stats
		beginTime := time.Now()
		defer func() {
			misc.HTTPDStats.Trigger(float64(time.Now().UnixNano() - beginTime.UnixNano()))
		}()
		// Shortcut - program-wide emergency lock-down
		if misc.EmergencyLockDown {
			/*
				An error response usually should carry status 5xx in this case, but the intention of
				emergency stop is to disable the program rather than crashing it and relaunching it.
				If an external trigger such as load balancer health check knocks on HTTP endpoint and restarts
				the program after consecutive HTTP failures, it would defeat the intention of emergency stop.
				Hence the status code here is OK.
			*/
			_, _ = w.Write([]byte(misc.ErrEmergencyLockDown.Error()))
			return
		}
		// Shortcut - rate limit check
		remoteIP := handler.GetRealClientIP(r)
		if !rateLimit.Add(remoteIP, true) {
			http.Error(w, "", http.StatusTooManyRequests)
			return
		}
		responseRecorder := &HTTPResponseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // the default status code written by a response writer is 200 OK
		}
		if restrictedRequestSize {
			// Lmit the maximum size of HTTP request body to be processed
			r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)
		}
		next.ServeHTTP(responseRecorder, r)
		handlerDuration := time.Since(beginTime)
		timeToFirstByteDuration := time.Since(beginTime)
		daemon.logger.Info("decoratedHandler", remoteIP, nil, "User-Agent \"%s\" referred by \"%s\", requested \"%s %s %s\", responded with code %d and %d bytes in %dus (time to 1st byte %dus)",
			r.Header.Get("User-Agent"), r.Header.Get("Referer"), r.Method, r.URL.EscapedPath(), r.Proto,
			responseRecorder.statusCode, responseRecorder.responseBodySize,
			handlerDuration.Microseconds(), timeToFirstByteDuration.Microseconds())
		// Give the observations of HTTP routine processing stats to prometheus
		if misc.EnablePrometheusIntegration {
			durationObs.Observe(handlerDuration.Seconds())
			timeToFirstByteObs.Observe(timeToFirstByteDuration.Seconds())
			responseSizeObs.Observe(float64(responseRecorder.responseBodySize))
		}
	}
	// Integrate the decorated handler with AWS x-ray. The crucial x-ray daemon program seems to be only capable of running on AWS compute resources.
	if misc.EnableAWSIntegration && inet.IsAWS() {
		return xray.Handler(xray.NewDynamicSegmentNamer("LaitosHTTPD", "*"), http.HandlerFunc(decoratedHandler))
	}
	return http.HandlerFunc(decoratedHandler)
}
