package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"ai-gateway/internal/limiter"
)

const (
	// maxRateLimitBodyPeek caps how much of the body we read to extract the
	// model field. 1 KiB is enough for the JSON header without buffering
	// huge chat histories.
	maxRateLimitBodyPeek = 64 * 1024
)

func RateLimit(keyLimiter, modelLimiter *limiter.TokenBucketLimiter, logger *slog.Logger, next http.Handler) http.Handler {
	if keyLimiter == nil && modelLimiter == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := extractAPIKey(r)

		if keyLimiter != nil {
			if !keyLimiter.Allow(apiKey) {
				http.Error(w, "rate limit exceeded for api key", http.StatusTooManyRequests)
				return
			}
		}

		if modelLimiter != nil {
			model, restored := extractModelFromBody(r)
			if model == "" {
				model = "unknown"
			}
			if !modelLimiter.Allow(model) {
				http.Error(w, "rate limit exceeded for model", http.StatusTooManyRequests)
				return
			}
			// Restore the body so downstream handlers can decode it normally.
			if restored != nil {
				r.Body = restored
			}
		}

		next.ServeHTTP(w, r)
	})
}

// extractModelFromBody peeks at the request body to find the OpenAI-style
// "model" field. It returns the model name and a replacement Body that callers
// must assign to r.Body so the handler chain can still read the original bytes.
//
// We use a length-bounded peek + io.MultiReader to avoid buffering the entire
// payload (chat histories can be megabytes).
func extractModelFromBody(r *http.Request) (string, io.ReadCloser) {
	if r.Body == nil || r.Body == http.NoBody {
		return "", nil
	}
	if r.ContentLength > maxRateLimitBodyPeek {
		// Still cheap: a streaming json.Decoder on a LimitReader stops as
		// soon as it sees "model". For very large requests we just give up
		// on per-model limiting; the per-key limiter still applies.
	}

	peek := make([]byte, maxRateLimitBodyPeek)
	n, _ := io.ReadFull(r.Body, peek)
	peek = peek[:n]

	// Reassemble: the bytes we already read + the rest of the body.
	restored := struct {
		io.Reader
		io.Closer
	}{
		Reader: io.MultiReader(bytes.NewReader(peek), r.Body),
		Closer: r.Body,
	}

	// Lightweight extraction: a streaming decoder finds the "model" key
	// without unmarshaling the whole document (chat histories don't need
	// to be parsed just to rate-limit).
	var probe struct {
		Model string `json:"model"`
	}
	_ = json.NewDecoder(bytes.NewReader(peek)).Decode(&probe)
	return probe.Model, io.NopCloser(restored)
}

func extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:]
	}
	return r.RemoteAddr
}
