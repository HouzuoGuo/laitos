package httpd

import (
	"net/http"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/httpd/handler"
	"github.com/HouzuoGuo/laitos/misc"
)

type middlewareResponseRecorder struct {
	http.ResponseWriter
	statusCode       int
	responseBodySize int
}

func (rec *middlewareResponseRecorder) WriteHeader(statusCode int) {
	rec.statusCode = statusCode
	rec.ResponseWriter.WriteHeader(statusCode)
}

func (rec *middlewareResponseRecorder) Write(b []byte) (int, error) {
	size, err := rec.ResponseWriter.Write(b)
	rec.responseBodySize += size
	return size, err
}

/*
DecorateWithMiddleware returns an HTTP middleware function that performs the following tasks:
- Limit the maximum size of HTTP request body to be processed.
- If emergency lock down is in effect, respond to client without invoking the actual handler function.
- Check client IP against rate limit.
- Log and record the duration of the request.
*/
func (daemon *Daemon) DecorateWithMiddleware(rateLimit *misc.RateLimit, restrictedRequestSize bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Lmit the maximum size of HTTP request body to be processed
		if restrictedRequestSize {
			r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)
		}
		// Record the duration of request handling in stats
		beginTimeNano := time.Now().UnixNano()
		defer func() {
			misc.HTTPDStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
		}()
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
		if rateLimit.Add(remoteIP, true) {
			responseRecorder := &middlewareResponseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // the default HTTP response status code is 200
			}
			next(responseRecorder, r)
			// Always close the request body
			if r.Body != nil {
				_ = r.Body.Close()
			}
		} else {
			http.Error(w, "", http.StatusTooManyRequests)
		}
		daemon.logger.Info("Handler", remoteIP, nil, "User-Agent \"%s\", requested \"%s %s %s\", response code %d, %d bytes, took %d milliseconds",
			r.Header.Get("User-Agent"), r.Method, r.URL.EscapedPath(), r.Proto,
			responseRecorder.statusCode, responseRecorder.responseBodySize, (time.Now().UnixNano()-beginTimeNano)/1000000)
	}
}
