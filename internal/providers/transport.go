package providers

import (
	"net"
	"net/http"
	"time"
)

// SharedTransport returns an optimized HTTP transport for LLM provider connections.
// It provides connection pooling suitable for concurrent orchestration workloads.
func SharedTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		MaxConnsPerHost:       0, // no limit
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}
}

// SharedHTTPClient returns an http.Client backed by SharedTransport with the given timeout.
func SharedHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: SharedTransport(),
		Timeout:   timeout,
	}
}
