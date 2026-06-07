package provider

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// UpstreamError captures everything the retry / breaker layer needs to classify
// an upstream failure without parsing error strings.
//
// Body is included only when small enough to be informative (capped by the
// caller); for very large bodies it is truncated. The original byte length is
// preserved in BodyLen for logging.
type UpstreamError struct {
	Provider   string
	StatusCode int
	Body       string
	BodyLen    int
	// RetryAfter is set when the upstream sent a Retry-After header. Zero
	// otherwise. The retry layer SHOULD honor it before applying its own backoff.
	RetryAfter time.Duration
}

func (e *UpstreamError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("upstream %s: status=%d retry-after=%s body=%s",
			e.Provider, e.StatusCode, e.RetryAfter, e.Body)
	}
	return fmt.Sprintf("upstream %s: status=%d body=%s", e.Provider, e.StatusCode, e.Body)
}

// IsRetryable reports whether the status code is one we should retry on.
// Errors with no upstream response (network / context) are NOT UpstreamError
// instances, so a separate caller-side check handles those.
func (e *UpstreamError) IsRetryable() bool {
	switch e.StatusCode {
	case http.StatusTooManyRequests,         // 429
		http.StatusBadGateway,                // 502
		http.StatusServiceUnavailable,        // 503
		http.StatusGatewayTimeout,            // 504
		http.StatusRequestTimeout:            // 408 — sometimes used by LB
		return true
	}
	return false
}

// AsUpstream returns the wrapped *UpstreamError if any, else nil. Convenience
// for callers that want to inspect status / Retry-After.
func AsUpstream(err error) *UpstreamError {
	var ue *UpstreamError
	if errors.As(err, &ue) {
		return ue
	}
	return nil
}

// parseRetryAfter accepts both delta-seconds and HTTP-date formats. It returns
// zero on parse failure — the caller's default backoff will then apply.
func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// newUpstreamErrorFromResp builds an UpstreamError, capping the body to
// maxBodyBytes so the wrapper doesn't blow up logs or memory for huge responses.
func newUpstreamErrorFromResp(providerName string, status int, body []byte, retryAfterHdr string, maxBodyBytes int) *UpstreamError {
	preview := body
	if len(preview) > maxBodyBytes {
		preview = preview[:maxBodyBytes]
	}
	return &UpstreamError{
		Provider:   providerName,
		StatusCode: status,
		Body:       string(preview),
		BodyLen:    len(body),
		RetryAfter: parseRetryAfter(retryAfterHdr),
	}
}

const maxErrorBodyBytes = 1024
