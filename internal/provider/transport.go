package provider

import (
	"net/http"
	"time"
)

// NewTransport creates an http.Transport with upstream connection pool limits.
// Pass a zero-value TransportConfig to get safe defaults (no limit = 0 means
// use Go defaults, which are unbounded — only set explicit values when > 0).
func NewTransport(maxConnsPerHost, maxIdleConnsPerHost, maxIdleConns int) *http.Transport {
	t := &http.Transport{
		IdleConnTimeout: 90 * time.Second,
	}
	if maxConnsPerHost > 0 {
		t.MaxConnsPerHost = maxConnsPerHost
	}
	if maxIdleConnsPerHost > 0 {
		t.MaxIdleConnsPerHost = maxIdleConnsPerHost
	}
	if maxIdleConns > 0 {
		t.MaxIdleConns = maxIdleConns
	}
	return t
}

// TransportSetter is implemented by providers that accept a shared transport.
type TransportSetter interface {
	SetTransport(*http.Transport)
}
