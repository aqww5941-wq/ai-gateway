package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"ai-gateway/internal/limiter"
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
			model := r.URL.Query().Get("model")
			if model == "" {
				model = "default"
			}
			if !modelLimiter.Allow(model) {
				http.Error(w, "rate limit exceeded for model", http.StatusTooManyRequests)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:]
	}
	return r.RemoteAddr
}
