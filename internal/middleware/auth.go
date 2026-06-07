package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

// Auth performs API-key authentication.
//
// Naive implementations compare the presented token to every configured key
// with ConstantTimeCompare — that's O(N) per request and an attacker who can
// time the loop iterations can infer the number of valid keys.
//
// We index keys by SHA-256 prefix for an O(1) candidate lookup, then run the
// constant-time compare only against the candidate. The SHA-256 lookup is itself
// constant-time over key contents (the hash is the same length for any input),
// so the original timing-attack property is preserved.
type Auth struct {
	hashes map[[32]byte][]byte
}

// NewAuth pre-hashes the configured keys for O(1) lookup. Returns nil when
// no keys are configured, in which case the middleware is bypassed.
func NewAuth(keys []string) *Auth {
	if len(keys) == 0 {
		return nil
	}
	a := &Auth{hashes: make(map[[32]byte][]byte, len(keys))}
	for _, k := range keys {
		a.hashes[sha256.Sum256([]byte(k))] = []byte(k)
	}
	return a
}

func (a *Auth) Wrap(logger *slog.Logger, next http.Handler) http.Handler {
	if a == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "unauthorized: missing api key", http.StatusUnauthorized)
			return
		}
		token := auth[7:]
		if !a.allow(token) {
			http.Error(w, "unauthorized: invalid api key", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *Auth) allow(token string) bool {
	digest := sha256.Sum256([]byte(token))
	candidate, ok := a.hashes[digest]
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), candidate) == 1
}
