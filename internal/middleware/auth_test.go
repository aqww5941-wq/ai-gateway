package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAuthNoKeys(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := Auth(nil, logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	keys := []string{"secret-key-123"}

	handler := Auth(keys, logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	keys := []string{"secret-key-123"}

	handler := Auth(keys, logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	keys := []string{"secret-key-123"}

	handler := Auth(keys, logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
