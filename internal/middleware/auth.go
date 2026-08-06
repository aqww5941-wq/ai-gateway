package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"ai-gateway/internal/store"
)

type ctxKey struct{}

var ctxKeyIdentity ctxKey

// IdentityFromCtx extracts the authenticated key identity from the request context.
func IdentityFromCtx(ctx context.Context) *store.KeyIdentity {
	id, ok := ctx.Value(ctxKeyIdentity).(*store.KeyIdentity)
	if !ok {
		return nil
	}
	return id
}

// Auth caches key identities from the store with periodic refresh.
type Auth struct {
	store   *store.Store
	cache   map[string]*store.KeyIdentity // token -> identity
	cacheMu sync.RWMutex
}

// NewAuth creates an auth middleware backed by the key store.
// Returns nil if store is nil.
// A background goroutine refreshes the in-memory cache every minute.
func NewAuth(s *store.Store) *Auth {
	if s == nil {
		return nil
	}
	a := &Auth{
		store: s,
		cache: make(map[string]*store.KeyIdentity),
	}
	a.refreshCache()
	go a.refreshLoop()
	return a
}

// NewAuthFromConfig creates an auth middleware backed by static config keys.
// Used as fallback when the store is unavailable.
func NewAuthFromConfig(keys []ConfigKey) *Auth {
	if len(keys) == 0 {
		return nil
	}
	a := &Auth{
		cache: make(map[string]*store.KeyIdentity),
	}
	for _, k := range keys {
		a.cache[k.Token] = &store.KeyIdentity{Name: k.Name, Role: k.Role, DailyLimit: k.DailyLimit, Models: k.Models}
	}
	return a
}

type ConfigKey struct {
	Token      string
	Name       string
	Role       string
	DailyLimit int64
	Models     string
}

func (a *Auth) refreshCache() {
	// Verify store is reachable; if not, keep stale cache.
	if _, err := a.store.ListKeys(); err != nil {
		return
	}
	a.cacheMu.Lock()
	a.cache = make(map[string]*store.KeyIdentity)
	a.cacheMu.Unlock()
}

func (a *Auth) refreshLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		a.refreshCache()
	}
}

// paths that skip auth: static assets and prometheus metrics.
var skipAuthPrefixes = []string{"/admin/dashboard/", "/metrics"}

func (a *Auth) Wrap(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, p := range skipAuthPrefixes {
			if strings.HasPrefix(r.URL.Path, p) {
				next.ServeHTTP(w, r)
				return
			}
		}

		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "unauthorized: missing api key", http.StatusUnauthorized)
			return
		}
		token := auth[7:]

		// Check cache first (covers config-based keys too).
		a.cacheMu.RLock()
		id, cached := a.cache[token]
		a.cacheMu.RUnlock()

		if !cached && a.store != nil {
			var err error
			id, err = a.store.LookupIdentity(token)
			if err != nil {
				logger.Error("auth lookup failed", "error", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			if id != nil {
				a.cacheMu.Lock()
				a.cache[token] = id
				a.cacheMu.Unlock()
			}
		}

		if id == nil {
			http.Error(w, "unauthorized: invalid api key", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyIdentity, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
