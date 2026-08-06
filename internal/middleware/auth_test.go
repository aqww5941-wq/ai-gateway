package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"ai-gateway/internal/store"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
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

	keys, err := st.ListKeys()
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("list keys count = %d, want 1", len(keys))
	}
	if err := st.SetKeyActive(keys[0].ID, false); err != nil {
		t.Fatalf("deactivate key: %v", err)
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

func TestAuthCurrentCacheDelaysRevocationUntilRefresh(t *testing.T) {
	st := newTestStore(t)
	token, err := st.CreateKey("cached", "user", "", 1000, 0)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	keys, err := st.ListKeys()
	if err != nil || len(keys) != 1 {
		t.Fatalf("list keys = %#v, %v", keys, err)
	}

	auth := NewAuth(st)
	called := 0
	handler := auth.Wrap(discardLogger(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusNoContent)
	}))
	serve := func() int {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder.Code
	}

	if status := serve(); status != http.StatusNoContent {
		t.Fatalf("initial auth status = %d", status)
	}
	if err := st.SetKeyActive(keys[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if status := serve(); status != http.StatusNoContent {
		t.Fatalf("cached auth status after revocation = %d, want current cached success", status)
	}
	auth.refreshCache()
	if status := serve(); status != http.StatusUnauthorized {
		t.Fatalf("auth status after refresh = %d, want 401", status)
	}
	if called != 2 {
		t.Fatalf("downstream calls = %d, want 2", called)
	}
	t.Log("known gap: cached credentials remain usable until refresh; Task 48 owns revocation semantics")
}

func TestAuthStoreFailureFailsClosed(t *testing.T) {
	st := newTestStore(t)
	auth := NewAuth(st)
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	called := false
	handler := auth.Wrap(discardLogger(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer invalid-after-close")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if called || recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Store failure result = called %v, status %d; want false, 500", called, recorder.Code)
	}
}

func TestIdentityFromCtx_NoAuth(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	id := IdentityFromCtx(req.Context())
	if id != nil {
		t.Error("expected nil identity when no auth set")
	}
}
