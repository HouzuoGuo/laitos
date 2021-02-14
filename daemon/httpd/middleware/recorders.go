package middleware

import (
	"bufio"
	"net"
	"net/http"
	"time"
)

// HTTPResponseRecorder is an http.ResponseWriter that comes with the capability of inspecting the size and timing characteristics of the HTTP response.
type HTTPResponseRecorder struct {
	http.Hijacker
	http.ResponseWriter
	statusCode           int
	totalWritten         int
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
	rec.totalWritten += size
	return size, err
}

// ConnRecorder is a net.Conn that remembers the size and timing characteristics of bytes written.
type ConnRecorder struct {
	net.Conn
	totalWritten         int
	timestampAtWriteCall time.Time
}

func (rec *ConnRecorder) Write(b []byte) (n int, err error) {
	if rec.timestampAtWriteCall.IsZero() {
		rec.timestampAtWriteCall = time.Now()
	}
	size, err := rec.Conn.Write(b)
	rec.totalWritten += size
	return size, err
}

// HTTPInterceptRecorder is an http.Hijacker that comes with the capability of inspecting the size and timing characteristics of the intercepted stream.
type HTTPInterceptRecorder struct {
	http.Hijacker
	ConnRecorder *ConnRecorder
}

// Hijack allows the caller to take over the HTTP connection.
func (rec *HTTPInterceptRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, writer, err := rec.Hijacker.Hijack()
	rec.ConnRecorder = &ConnRecorder{Conn: conn}
	return rec.ConnRecorder, writer, err
}
