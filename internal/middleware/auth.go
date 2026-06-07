package middleware

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

func Auth(keys []string, logger *slog.Logger, next http.Handler) http.Handler {
	if len(keys) == 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "unauthorized: missing api key", http.StatusUnauthorized)
			return
		}
		token := auth[7:]

		valid := false
		for _, k := range keys {
			if subtle.ConstantTimeCompare([]byte(token), []byte(k)) == 1 {
				valid = true
				break
			}
		}
		if !valid {
			http.Error(w, "unauthorized: invalid api key", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
