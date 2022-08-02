package httpproxy

import (
	"io"
	"net"
	"net/http"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/httpd/middleware"
	"github.com/HouzuoGuo/laitos/misc"
)

// httpTransport is the default implementation of HTTP transport (and round tripper) used by proxy handler for
// HTTP proxy requests.
// This transport is not involved in handling HTTPS proxy requests.
var httpTransport = &http.Transport{
	// Do not support usage of an upstream HTTP proxy
	Proxy: nil,
	DialContext: (&net.Dialer{
		Timeout:   IOTimeout,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}).DialContext,
	ForceAttemptHTTP2:     false,
	MaxIdleConns:          100,
	IdleConnTimeout:       IOTimeout,
	TLSHandshakeTimeout:   IOTimeout,
	ExpectContinueTimeout: 1 * time.Second,
}

// ProxyHandler is an HTTP handler function that implements an HTTP proxy capable of handling HTTPS as well.
func (daemon Daemon) ProxyHandler(w http.ResponseWriter, r *http.Request) {
	// Pass the intended destination through DNS daemon's blacklist filter
	if daemon.DNSDaemon != nil && daemon.DNSDaemon.IsInBlacklist(r.Host) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	clientIP := middleware.GetRealClientIP(r)
	switch r.Method {
	case http.MethodConnect:
		// Open a connection to the destination and then entirely hand over the connection to the client
		dstConn, err := net.DialTimeout("tcp", r.Host, IOTimeout)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		// OK to CONNECT
		w.WriteHeader(http.StatusOK)
		// Data stream follows
		hijackedStream, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "", http.StatusInternalServerError)
			daemon.logger.Warning("ProxyHandler", clientIP, nil, "connection stream cannot be tapped into")
			return
		}
		reqConn, _, err := hijackedStream.Hijack()
		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			daemon.logger.Warning("ProxyHandler", clientIP, err, "failed to tap into HTTP connection stream")
			return
		}
		// Apply optimistaions to the inner-most TCP connection that may have been wrapped around several layers of connection recorder middleware
		innerMostReqConn := reqConn
		for {
			if connRecorder, ok := innerMostReqConn.(*middleware.ConnRecorder); ok {
				innerMostReqConn = connRecorder.Conn
			} else {
				break
			}
		}
		misc.TweakTCPConnection(innerMostReqConn.(*net.TCPConn), IOTimeout)
		misc.TweakTCPConnection(dstConn.(*net.TCPConn), IOTimeout)
		go misc.PipeConn(daemon.logger, true, IOTimeout, 1280, dstConn, reqConn)
		misc.PipeConn(daemon.logger, true, IOTimeout, 1280, reqConn, dstConn)
	default:
		// Execute the request as-is without handling higher-level mechanisms such as cookies and redirects
		resp, err := httpTransport.RoundTrip(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		// Copy all received headers to the client
		for key, vals := range resp.Header {
			for _, val := range vals {
				w.Header().Add(key, val)
			}
		}
		// Copy status code and response body to the client
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			daemon.logger.Warning("ProxyHandler", clientIP, err, "failed to copy response body back to client")
		}
	}
}

// CheckClientIPMiddleware decorates the HTTP handler with an additional check of client IP address, and calls
// the next handler only if the client IP is among the allowed.
// If the client IP is not allowed, the decorated HTTP handler will respond politely with an appropriate HTTP
// status code.
func (daemon *Daemon) CheckClientIPMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientIP := net.ParseIP(middleware.GetRealClientIP(r))
		// Either the client has already successfully reported to the store&forward message processor of this server...
		if daemon.CommandProcessor.Features.MessageProcessor.HasClientTag(clientIP.String()) {
			next(w, r)
		} else {
			// Or the IP is among the allowed CIDR blocks
			for _, allowedCIDR := range daemon.allowFromIPNets {
				if allowedCIDR.Contains(clientIP) {
					next(w, r)
					return
				}
			}
			http.Error(w, "Your IP is not allowed to use this HTTP proxy server", http.StatusForbidden)
			daemon.logger.Warning("CheckClientIPMiddleware", clientIP.String(), nil, "the client IP is not among the allowed CIDRs")
		}
	}
}
