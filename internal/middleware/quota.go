package middleware

import (
	"log/slog"
	"net/http"
	"strings"

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
