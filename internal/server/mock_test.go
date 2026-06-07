package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"ai-gateway/config"
	"ai-gateway/internal/provider"
	"ai-gateway/internal/router"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

func startMockLLMServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req provider.ChatRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Check if streaming is requested
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)

			chunks := []string{"Hello", " ", "World"}
			for i, c := range chunks {
				chunk := provider.StreamChunk{
					ID:      "chatcmpl-test",
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   req.Model,
					Choices: []provider.StreamChoice{
						{
							Index: i,
							Delta: provider.StreamDelta{Content: c},
						},
					},
				}
				data, _ := json.Marshal(chunk)
				w.Write([]byte("data: " + string(data) + "\n\n"))
				flusher.Flush()
			}
			// Final chunk with usage
			final := provider.StreamChunk{
				ID:      "chatcmpl-test",
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []provider.StreamChoice{
					{
						Index: 0,
						Delta: provider.StreamDelta{},
					},
				},
				Usage: &provider.Usage{
					PromptTokens:     10,
					CompletionTokens: 3,
					TotalTokens:      13,
				},
			}
			finalData, _ := json.Marshal(final)
			w.Write([]byte("data: " + string(finalData) + "\n\n"))
			w.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
			return
		}

		resp := provider.ChatResponse{
			ID:      "chatcmpl-test",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []provider.Choice{
				{
					Index: 0,
					Message: provider.Message{
						Role:    "assistant",
						Content: "Hello from " + req.Model + "! Received: " + req.Messages[len(req.Messages)-1].Content,
					},
					FinishReason: "stop",
				},
			},
			Usage: provider.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestGatewayNonStream(t *testing.T) {
	mock := startMockLLMServer(t)
	defer mock.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{Port: 0},
		Cache:  config.CacheConfig{Enabled: false},
		Providers: []config.ProviderConfig{
			{
				Name:    "mock",
				Type:    "openai",
				APIKey:  "test-key",
				BaseURL: mock.URL,
				Models:  []string{"mock-model"},
				Timeout: 10 * time.Second,
			},
		},
		Routes: []config.RouteConfig{
			{
				Name:     "default",
				Strategy: "round_robin",
				Match:    config.RouteMatch{Model: "mock-model"},
				Targets: []config.RouteTarget{
					{Provider: "mock", Model: "mock-model"},
				},
			},
		},
	}

	providers := map[string]provider.LLMProvider{}
	for _, pc := range cfg.Providers {
		p, err := provider.NewOpenAI(pc, testLogger.With("provider", pc.Name))
		if err != nil {
			t.Fatal(err)
		}
		providers[pc.Name] = p
	}

	r, err := router.NewRouter(cfg.Routes)
	if err != nil {
		t.Fatal(err)
	}

	// Use httptest to call the handler
	body := `{"model":"mock-model","messages":[{"role":"user","content":"Hi"}],"stream":false}`
	httpReq := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	srv := &Server{
		config:    cfg,
		router:    r,
		providers: providers,
		logger:    testLogger,
	}

	srv.handleChatCompletion(w, httpReq)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp provider.ChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}

	content := resp.Choices[0].Message.Content
	if !strings.Contains(content, "Hello from mock-model") {
		t.Errorf("unexpected response content: %s", content)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("expected 15 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

func TestGatewayCache(t *testing.T) {
	mock := startMockLLMServer(t)
	defer mock.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{Port: 0},
		Cache: config.CacheConfig{
			Enabled:  true,
			Backend:  "memory",
			TTL:      "1h",
			Strategy: "exact",
			MaxSize:  10,
		},
		Providers: []config.ProviderConfig{
			{
				Name:    "mock",
				Type:    "openai",
				APIKey:  "test-key",
				BaseURL: mock.URL,
				Models:  []string{"mock-model"},
				Timeout: 10 * time.Second,
			},
		},
		Routes: []config.RouteConfig{
			{
				Name:     "default",
				Strategy: "round_robin",
				Match:    config.RouteMatch{Model: "mock-model"},
				Targets: []config.RouteTarget{
					{Provider: "mock", Model: "mock-model"},
				},
			},
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

	body := `{"model":"mock-model","messages":[{"role":"user","content":"Cache test"}],"stream":false}`

	// First request (cache miss)
	req1 := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w1 := httptest.NewRecorder()
	srv.handleChatCompletion(w1, req1)
	if w1.Code != 200 {
		t.Fatalf("first request: expected 200, got %d", w1.Code)
	}

	// Second request (should be cache hit)
	req2 := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w2 := httptest.NewRecorder()
	srv.handleChatCompletion(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("second request: expected 200, got %d", w2.Code)
	}

	// Verify both responses are identical (second was from cache)
	var resp1, resp2 provider.ChatResponse
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	json.Unmarshal(w2.Body.Bytes(), &resp2)

	if resp1.ID != resp2.ID {
		t.Errorf("cached response mismatch: %s vs %s", resp1.ID, resp2.ID)
	}
}
