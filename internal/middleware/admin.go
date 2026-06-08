package middleware

import (
	"net/http"
)

// AdminOnly returns middleware that restricts access to admin-role keys only.
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := IdentityFromCtx(r.Context())
		if id == nil || !id.IsAdmin() {
			http.Error(w, "forbidden: admin role required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
