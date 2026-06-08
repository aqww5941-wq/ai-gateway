package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-gateway/internal/store"
)

func isSystemPath(path string) bool {
	return strings.HasPrefix(path, "/admin") || strings.HasPrefix(path, "/metrics")
}

func QuotaCheck(s *store.Store, logger *slog.Logger, next http.Handler) http.Handler {
	if s == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSystemPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		identity := IdentityFromCtx(r.Context())
		if identity == nil {
			next.ServeHTTP(w, r)
			return
		}
		allowed, used, limit, err := s.CheckQuota(identity.ID)
		if err != nil {
			logger.Error("quota check failed", "error", err)
			next.ServeHTTP(w, r) // fail open
			return
		}

		setQuotaHeaders(w, limit, limit-used, nextDailyReset())
		if !allowed {
			logger.Warn("quota exceeded",
				"key", identity.Name,
				"used", used,
				"limit", limit,
			)
			http.Error(w, "quota exceeded: daily token budget exhausted", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func setQuotaHeaders(w http.ResponseWriter, limit, remaining int64, resetAt int64) {
	if limit <= 0 {
		return
	}
	if remaining < 0 {
		remaining = 0
	}
	w.Header().Set("X-RateLimit-Limit-Requests", strconv.FormatInt(limit, 10))
	w.Header().Set("X-RateLimit-Remaining-Requests", strconv.FormatInt(remaining, 10))
	w.Header().Set("X-RateLimit-Reset-Requests", strconv.FormatInt(resetAt, 10))
}

func nextDailyReset() int64 {
	now := time.Now().UTC()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	return tomorrow.Unix()
}
