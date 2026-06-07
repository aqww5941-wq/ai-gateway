package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestAuthNoKeys(t *testing.T) {
	logger := newTestLogger()

	handler := NewAuth(nil).Wrap(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthValidKey(t *testing.T) {
	logger := newTestLogger()
	keys := []string{"secret-key-123"}

	handler := NewAuth(keys).Wrap(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer secret-key-123")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthInvalidKey(t *testing.T) {
	logger := newTestLogger()
	keys := []string{"secret-key-123"}

	handler := NewAuth(keys).Wrap(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	logger := newTestLogger()
	keys := []string{"secret-key-123"}

	handler := NewAuth(keys).Wrap(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMultipleKeysO1(t *testing.T) {
	// Make sure correctness holds for many keys (O(1) lookup path).
	keys := make([]string, 1000)
	for i := range keys {
		keys[i] = "k-" + string(rune('a'+i%26)) + "-" + string(rune('0'+i%10))
	}
	keys = append(keys, "winner")
	a := NewAuth(keys)
	if !a.allow("winner") {
		t.Fatal("valid key rejected")
	}
	if a.allow("not-present") {
		t.Fatal("invalid key accepted")
	}
}
