package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ai-gateway/config"
	"ai-gateway/internal/breaker"
	"ai-gateway/internal/provider"
	"ai-gateway/internal/router"
)

// TestBreakerOpensAfterUpstreamFailures — a chain of 503s should trip the
// breaker, after which subsequent requests must short-circuit with 503 (not
// hit the upstream at all).
func TestBreakerOpensAfterUpstreamFailures(t *testing.T) {
	var upstreamCalls atomic.Int32
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		http.Error(w, `{"error":"overloaded"}`, http.StatusServiceUnavailable)
	}))
	defer mock.Close()

	srv := newTestSrvSingleProvider(t, mock.URL)
	// Tighten the breaker so the test is fast and deterministic.
	srv.currentSnapshot().breakers = breaker.NewManager(breaker.Config{
		FailureThreshold: 3,
		CoolDown:         50 * time.Millisecond,
		HalfOpenSuccess:  1,
	})
	// Single attempt — we want every user request to count as exactly one
	// failure for breaker accounting, not three.
	srv.retryPolicy.MaxAttempts = 1

	body := `{"model":"mock-model","messages":[{"role":"user","content":"Hi"}],"stream":false}`

	// Drive 3 failing requests to trip the breaker.
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.handleChatCompletion(w, req)
		if w.Code != http.StatusBadGateway && w.Code != http.StatusServiceUnavailable {
			t.Fatalf("req %d: expected 502/503, got %d", i, w.Code)
		}
	}
	preTripCalls := upstreamCalls.Load()
	if preTripCalls != 3 {
		t.Fatalf("expected 3 upstream calls before trip, got %d", preTripCalls)
	}

	// 4th request must be short-circuited.
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleChatCompletion(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("post-trip: expected 503, got %d", w.Code)
	}
	if got := upstreamCalls.Load(); got != preTripCalls {
		t.Fatalf("breaker should have short-circuited, but upstream was hit again: %d → %d", preTripCalls, got)
	}
}

// TestBreakerRecoversAfterCooldown — once the upstream heals AND the
// cool-down elapses, the breaker should admit a probe, succeed, and close.
func TestBreakerRecoversAfterCooldown(t *testing.T) {
	var healthy atomic.Bool
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !healthy.Load() {
			http.Error(w, `{"error":"down"}`, http.StatusServiceUnavailable)
			return
		}
		resp := provider.ChatResponse{
			ID:      "ok",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "mock-model",
			Choices: []provider.Choice{{Index: 0, Message: provider.Message{Role: "assistant", Content: "ok"}, FinishReason: "stop"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mock.Close()

	srv := newTestSrvSingleProvider(t, mock.URL)
	srv.currentSnapshot().breakers = breaker.NewManager(breaker.Config{
		FailureThreshold: 2,
		CoolDown:         100 * time.Millisecond,
		HalfOpenSuccess:  1,
	})
	srv.retryPolicy.MaxAttempts = 1

	body := `{"model":"mock-model","messages":[{"role":"user","content":"x"}],"stream":false}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
		srv.handleChatCompletion(httptest.NewRecorder(), req)
	}
	// Breaker should be open now.
	if br := srv.currentSnapshot().breakers.Get("mock"); br.State() != breaker.StateOpen {
		t.Fatalf("breaker state = %s, want open", br.State())
	}
	healthy.Store(true)
	time.Sleep(150 * time.Millisecond)

	// Probe attempt — succeeds, closes the breaker.
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleChatCompletion(w, req)
	if w.Code != 200 {
		t.Fatalf("probe expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if br := srv.currentSnapshot().breakers.Get("mock"); br.State() != breaker.StateClosed {
		t.Fatalf("breaker state = %s, want closed (after successful probe)", br.State())
	}
}

// TestRetriesThenSucceeds — upstream fails twice then succeeds; the retry
// helper should expose the eventual success to the caller and leave the
// breaker closed.
func TestRetriesThenSucceeds(t *testing.T) {
	var n atomic.Int32
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) < 3 {
			http.Error(w, `{"error":"flaky"}`, http.StatusServiceUnavailable)
			return
		}
		resp := provider.ChatResponse{
			ID:      "ok",
			Object:  "chat.completion",
			Model:   "mock-model",
			Choices: []provider.Choice{{Index: 0, Message: provider.Message{Role: "assistant", Content: "ok"}, FinishReason: "stop"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mock.Close()

	srv := newTestSrvSingleProvider(t, mock.URL)
	srv.retryPolicy.MaxAttempts = 3
	srv.retryPolicy.BaseDelay = time.Millisecond
	srv.retryPolicy.MaxDelay = 5 * time.Millisecond

	body := `{"model":"mock-model","messages":[{"role":"user","content":"x"}],"stream":false}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleChatCompletion(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 after retry, got %d: %s", w.Code, w.Body.String())
	}
	if got := n.Load(); got != 3 {
		t.Fatalf("upstream attempts = %d, want 3", got)
	}
	if br := srv.currentSnapshot().breakers.Get("mock"); br.State() != breaker.StateClosed {
		t.Fatalf("breaker state after retried success = %s, want closed", br.State())
	}
}

// TestSingleflightDeduplicatesConcurrentCacheMisses — N goroutines firing
// the same prompt in parallel should produce exactly 1 upstream call.
//
// This is the singleflight win: a popular prompt arriving at the same time
// to a cold cache used to cause N upstream calls; now it causes 1.
func TestSingleflightDeduplicatesConcurrentCacheMisses(t *testing.T) {
	var upstreamCalls atomic.Int32
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		// Sleep so concurrent callers actually pile up behind one in-flight call.
		time.Sleep(50 * time.Millisecond)
		resp := provider.ChatResponse{
			ID:      "ok",
			Object:  "chat.completion",
			Model:   "mock-model",
			Choices: []provider.Choice{{Index: 0, Message: provider.Message{Role: "assistant", Content: "ok"}, FinishReason: "stop"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mock.Close()

	srv := newTestSrvSingleProviderWithCache(t, mock.URL, false /* no cache so we actually exercise singleflight */)

	body := `{"model":"mock-model","messages":[{"role":"user","content":"same"}],"stream":false}`
	const n = 30
	done := make(chan int, n)
	for i := 0; i < n; i++ {
		go func() {
			req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
			w := httptest.NewRecorder()
			srv.handleChatCompletion(w, req)
			done <- w.Code
		}()
	}
	for i := 0; i < n; i++ {
		code := <-done
		if code != 200 {
			t.Errorf("call %d: code=%d", i, code)
		}
	}
	if got := upstreamCalls.Load(); got != 1 {
		t.Fatalf("singleflight failed: upstream called %d times, want 1", got)
	}
}

// TestNonRetryableErrorNotRetried — a 400 should be returned immediately
// without burning extra attempts or tripping the breaker.
func TestNonRetryableErrorNotRetried(t *testing.T) {
	var calls atomic.Int32
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
	}))
	defer mock.Close()

	srv := newTestSrvSingleProvider(t, mock.URL)
	srv.retryPolicy.MaxAttempts = 3

	body := `{"model":"mock-model","messages":[{"role":"user","content":"x"}],"stream":false}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleChatCompletion(w, req)

	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream called %d times for a 400, want 1 (no retry)", got)
	}
	if br := srv.currentSnapshot().breakers.Get("mock"); br.State() != breaker.StateClosed {
		t.Fatalf("breaker state = %s, want closed (400 is client-fault, not upstream-fault)", br.State())
	}
}

// TestStatusForMapping — ErrOpen maps to 503, upstream 429 maps to 429, rest to 502.
func TestStatusForMapping(t *testing.T) {
	srv := &Server{}
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"breaker open", breaker.ErrOpen, http.StatusServiceUnavailable},
		{"upstream 429", &provider.UpstreamError{StatusCode: 429}, http.StatusTooManyRequests},
		{"upstream 500", &provider.UpstreamError{StatusCode: 500}, http.StatusBadGateway},
		{"generic err", fmt.Errorf("boom"), http.StatusBadGateway},
		{"wrapped breaker err", fmt.Errorf("oops: %w", breaker.ErrOpen), http.StatusServiceUnavailable},
		{"ctx err", context.Canceled, http.StatusBadGateway},
	}
	for _, c := range cases {
		if got := srv.statusFor(c.err); got != c.want {
			t.Errorf("%s: statusFor = %d, want %d", c.name, got, c.want)
		}
	}
	_ = errors.New // keep errors import
}

// --- helpers ---

func newTestSrvSingleProvider(t *testing.T, baseURL string) *Server {
	return newTestSrvSingleProviderWithCache(t, baseURL, false)
}

func newTestSrvSingleProviderWithCache(t *testing.T, baseURL string, withCache bool) *Server {
	t.Helper()
	cacheCfg := config.CacheConfig{Enabled: false}
	if withCache {
		cacheCfg = config.CacheConfig{Enabled: true, Backend: "memory", TTL: "1h", Strategy: "exact", MaxSize: 10}
	}
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 0},
		Cache:  cacheCfg,
		Providers: []config.ProviderConfig{
			{Name: "mock", Type: "openai", APIKey: "test-key", BaseURL: baseURL, Models: []string{"mock-model"}, Timeout: 5 * time.Second},
		},
		Routes: []config.RouteConfig{
			{Name: "default", Strategy: "round_robin", Match: config.RouteMatch{Model: "mock-model"}, Targets: []config.RouteTarget{{Provider: "mock", Model: "mock-model"}}},
		},
	}
	r, _ := router.NewRouter(cfg.Routes)
	providers := map[string]provider.LLMProvider{}
	for _, pc := range cfg.Providers {
		p, _ := provider.NewOpenAI(pc, testLogger.With("provider", pc.Name))
		providers[pc.Name] = p
	}
	srv, err := New(cfg, r, providers, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}
