package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"ai-gateway/internal/store"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestAuthValidKey(t *testing.T) {
	st := newTestStore(t)
	token, err := st.CreateKey("dev", "user", "", 1000, 0)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	logger := newTestLogger()
	auth := NewAuth(st)
	handler := auth.Wrap(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := IdentityFromCtx(r.Context())
		if id == nil {
			t.Error("expected identity in context")
		} else if id.Name != "dev" {
			t.Errorf("expected name dev, got %s", id.Name)
		}
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthInvalidKey(t *testing.T) {
	st := newTestStore(t)
	_, err := st.CreateKey("dev", "user", "", 1000, 0)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	logger := newTestLogger()
	handler := NewAuth(st).Wrap(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthNoHeader(t *testing.T) {
	st := newTestStore(t)
	_, err := st.CreateKey("dev", "user", "", 1000, 0)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	logger := newTestLogger()
	handler := NewAuth(st).Wrap(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthInactiveKey(t *testing.T) {
	st := newTestStore(t)
	token, err := st.CreateKey("dev", "user", "", 1000, 0)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	// Deactivate
	keys, _ := st.ListKeys()
	if len(keys) > 0 {
		st.SetKeyActive(keys[0].ID, false)
	}

	logger := newTestLogger()
	handler := NewAuth(st).Wrap(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401 for inactive key, got %d", w.Code)
	}
}

func TestIdentityFromCtx_NoAuth(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	id := IdentityFromCtx(req.Context())
	if id != nil {
		t.Error("expected nil identity when no auth set")
	}
}
