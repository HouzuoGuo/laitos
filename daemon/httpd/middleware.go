package httpd

import (
	"net/http"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/httpd/handler"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/aws/aws-xray-sdk-go/xray"
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

/*
DecorateWithMiddleware returns an HTTP middleware handler that performs the following tasks:
- Limit the maximum size of HTTP request body to be processed.
- If emergency lock down is in effect, respond to client without invoking the actual handler function.
- Check client IP against rate limit.
- Log and record the duration of the request.
- Integrate with AWS x-ray.
*/
func (daemon *Daemon) DecorateWithMiddleware(rateLimit *misc.RateLimit, restrictedRequestSize bool, next http.HandlerFunc) http.Handler {
	decoratedHandler := func(w http.ResponseWriter, r *http.Request) {
		// Record the duration of request handling in stats
		beginTimeNano := time.Now().UnixNano()
		defer func() {
			misc.HTTPDStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
		}()
		// Lmit the maximum size of HTTP request body to be processed
		if restrictedRequestSize {
			r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)
		}
		if misc.EmergencyLockDown {
			/*
				An error response usually should carry status 5xx in this case, but the intention of
				emergency stop is to disable the program rather than crashing it and relaunching it.
				If an external trigger such as load balancer health check knocks on HTTP endpoint and relaunches
				the program after consecutive HTTP failures, it would defeat the intention of emergency stop.
				Hence the status code here is OK.
			*/
			_, _ = w.Write([]byte(misc.ErrEmergencyLockDown.Error()))
			misc.HTTPDStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
			return
		}
		// Check client IP against rate limit
		remoteIP := handler.GetRealClientIP(r)
		responseRecorder := &HTTPResponseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // the default status code written by a response writer is 200 OK
		}
		if rateLimit.Add(remoteIP, true) {
			next.ServeHTTP(responseRecorder, r)
			// Always close the request body
			if r.Body != nil {
				_ = r.Body.Close()
			}
		} else {
			http.Error(w, "", http.StatusTooManyRequests)
		}
		daemon.logger.Info("Handler", remoteIP, nil, "User-Agent \"%s\" referred by \"%s\", requested \"%s %s %s\", responded with code %d and %d bytes in %dus (time to 1st byte %dus)",
			r.Header.Get("User-Agent"), r.Header.Get("Referer"), r.Method, r.URL.EscapedPath(), r.Proto,
			responseRecorder.statusCode, responseRecorder.responseBodySize,
			(time.Now().UnixNano()-beginTimeNano)/1000, (time.Now().UnixNano()-responseRecorder.timestampAtWriteCall.UnixNano())/1000)
	}
	// Integrate the decorated handler with AWS x-ray
	if misc.EnableAWSIntegration && inet.IsAWS() {
		return xray.Handler(xray.NewDynamicSegmentNamer("LaitosHTTPD", "*"), http.HandlerFunc(decoratedHandler))
	}
	return http.HandlerFunc(decoratedHandler)
}
