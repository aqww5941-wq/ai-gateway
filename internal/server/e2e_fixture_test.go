package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ai-gateway/config"
	"ai-gateway/internal/provider"
	"ai-gateway/internal/router"
)

const fixtureClientCredential = "fixture-client-credential"

type legacyUpstreamObservation struct {
	method string
	path   string
	authOK bool
	body   provider.ChatRequest
}

func TestLegacyGatewayUnaryEndToEndContract(t *testing.T) {
	observed := make(chan legacyUpstreamObservation, 1)
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		var body provider.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode upstream request: %v", err)
			http.Error(w, "invalid fixture request", http.StatusBadRequest)
			return
		}
		observed <- legacyUpstreamObservation{
			method: r.Method,
			path:   r.URL.Path,
			authOK: r.Header.Get("Authorization") == "Bearer fixture-primary-credential",
			body:   body,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(provider.ChatResponse{
			ID:      "fixture-unary-response",
			Object:  "chat.completion",
			Model:   body.Model,
			Choices: []provider.Choice{{Index: 0, Message: provider.Message{Role: "assistant", Content: "fixture-output"}, FinishReason: "stop"}},
			Usage:   provider.Usage{PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6},
		})
	}))
	t.Cleanup(upstream.Close)

	gateway := newLegacyGatewayFixture(t,
		[]config.ProviderConfig{legacyProviderConfig("primary", "fixture-primary-credential", upstream.URL, "fixture-upstream-model")},
		config.RouteConfig{Name: "fixture-route", Strategy: "round_robin", Match: config.RouteMatch{Model: "fixture-virtual-model"}, Targets: []config.RouteTarget{{Provider: "primary", Model: "fixture-upstream-model"}}},
		"fixture-virtual-model",
	)
	endpoint := httptest.NewServer(gateway.httpSrv.Handler)
	t.Cleanup(endpoint.Close)

	response := doLegacyGatewayRequest(t, endpoint.URL, fixtureClientCredential, `{"model":"fixture-virtual-model","messages":[{"role":"user","content":"fixture-input"}],"temperature":0.25,"max_tokens":32}`)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("gateway status = %d, body length = %d", response.StatusCode, len(body))
	}
	var decoded provider.ChatResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != "fixture-unary-response" || decoded.Model != "fixture-upstream-model" || decoded.Usage.TotalTokens != 6 || decoded.Choices[0].Message.Content != "fixture-output" {
		t.Fatalf("gateway response = %#v", decoded)
	}

	request := <-observed
	if upstreamCalls.Load() != 1 || request.method != http.MethodPost || request.path != "/chat/completions" || !request.authOK {
		t.Fatalf("upstream contract = calls %d, method %q, path %q, authorization valid %v", upstreamCalls.Load(), request.method, request.path, request.authOK)
	}
	if request.body.Model != "fixture-upstream-model" || request.body.Stream || len(request.body.Messages) != 1 || request.body.Messages[0].Content != "fixture-input" {
		t.Fatalf("routed upstream body = %#v", request.body)
	}
}

func TestLegacyGatewayAuthAndModelPolicyStopBeforeUpstream(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		http.Error(w, "unexpected fixture call", http.StatusInternalServerError)
	}))
	t.Cleanup(upstream.Close)

	gateway := newLegacyGatewayFixture(t,
		[]config.ProviderConfig{legacyProviderConfig("primary", "fixture-primary-credential", upstream.URL, "fixture-upstream-model")},
		config.RouteConfig{Name: "fixture-route", Strategy: "round_robin", Match: config.RouteMatch{Model: "fixture-virtual-model"}, Targets: []config.RouteTarget{{Provider: "primary", Model: "fixture-upstream-model"}}},
		"fixture-virtual-model",
	)
	endpoint := httptest.NewServer(gateway.httpSrv.Handler)
	t.Cleanup(endpoint.Close)

	tests := []struct {
		name       string
		credential string
		model      string
		wantStatus int
	}{
		{name: "missing credential", model: "fixture-virtual-model", wantStatus: http.StatusUnauthorized},
		{name: "invalid credential", credential: "fixture-invalid-credential", model: "fixture-virtual-model", wantStatus: http.StatusUnauthorized},
		{name: "model forbidden", credential: fixtureClientCredential, model: "fixture-forbidden-model", wantStatus: http.StatusForbidden},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			response := doLegacyGatewayRequest(t, endpoint.URL, testCase.credential, `{"model":"`+testCase.model+`","messages":[{"role":"user","content":"fixture-input"}]}`)
			defer response.Body.Close()
			if response.StatusCode != testCase.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, testCase.wantStatus)
			}
		})
	}
	if upstreamCalls.Load() != 0 {
		t.Fatalf("rejected requests reached upstream %d times", upstreamCalls.Load())
	}
}

func TestLegacyGatewayRetriesThenFallsBack(t *testing.T) {
	var primaryCalls atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls.Add(1)
		http.Error(w, `{"error":"fixture_unavailable"}`, http.StatusServiceUnavailable)
	}))
	t.Cleanup(primary.Close)

	var secondaryCalls atomic.Int32
	secondaryObserved := make(chan legacyUpstreamObservation, 1)
	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondaryCalls.Add(1)
		var body provider.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode fallback request: %v", err)
			return
		}
		secondaryObserved <- legacyUpstreamObservation{method: r.Method, path: r.URL.Path, authOK: r.Header.Get("Authorization") == "Bearer fixture-secondary-credential", body: body}
		_ = json.NewEncoder(w).Encode(provider.ChatResponse{
			ID:      "fixture-fallback-response",
			Object:  "chat.completion",
			Model:   body.Model,
			Choices: []provider.Choice{{Index: 0, Message: provider.Message{Role: "assistant", Content: "fixture-fallback-output"}, FinishReason: "stop"}},
		})
	}))
	t.Cleanup(secondary.Close)

	gateway := newLegacyGatewayFixture(t,
		[]config.ProviderConfig{
			legacyProviderConfig("primary", "fixture-primary-credential", primary.URL, "fixture-primary-model"),
			legacyProviderConfig("secondary", "fixture-secondary-credential", secondary.URL, "fixture-secondary-model"),
		},
		config.RouteConfig{Name: "fixture-fallback", Strategy: "fallback", Match: config.RouteMatch{Model: "fixture-virtual-model"}, Targets: []config.RouteTarget{{Provider: "primary", Model: "fixture-primary-model"}, {Provider: "secondary", Model: "fixture-secondary-model"}}},
		"fixture-virtual-model",
	)
	gateway.retryPolicy.MaxAttempts = 2
	gateway.retryPolicy.BaseDelay = time.Millisecond
	gateway.retryPolicy.MaxDelay = 2 * time.Millisecond
	endpoint := httptest.NewServer(gateway.httpSrv.Handler)
	t.Cleanup(endpoint.Close)

	response := doLegacyGatewayRequest(t, endpoint.URL, fixtureClientCredential, `{"model":"fixture-virtual-model","messages":[{"role":"user","content":"fixture-input"}]}`)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("fallback status = %d", response.StatusCode)
	}
	var decoded provider.ChatResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != "fixture-fallback-response" || decoded.Model != "fixture-secondary-model" {
		t.Fatalf("fallback response = %#v", decoded)
	}
	request := <-secondaryObserved
	if primaryCalls.Load() != 2 || secondaryCalls.Load() != 1 || !request.authOK || request.body.Model != "fixture-secondary-model" {
		t.Fatalf("fallback attempts = primary %d, secondary %d, secondary auth valid %v, model %q", primaryCalls.Load(), secondaryCalls.Load(), request.authOK, request.body.Model)
	}
}

func TestLegacyGatewayCurrentFallbackContinuesAfterUpstream400(t *testing.T) {
	var primaryCalls atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls.Add(1)
		http.Error(w, `{"error":"fixture_invalid_request"}`, http.StatusBadRequest)
	}))
	t.Cleanup(primary.Close)

	var secondaryCalls atomic.Int32
	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondaryCalls.Add(1)
		_ = json.NewEncoder(w).Encode(provider.ChatResponse{ID: "fixture-current-fallback", Object: "chat.completion", Model: "fixture-secondary-model", Choices: []provider.Choice{{Index: 0, Message: provider.Message{Role: "assistant", Content: "fixture-output"}}}})
	}))
	t.Cleanup(secondary.Close)

	gateway := newLegacyGatewayFixture(t,
		[]config.ProviderConfig{
			legacyProviderConfig("primary", "fixture-primary-credential", primary.URL, "fixture-primary-model"),
			legacyProviderConfig("secondary", "fixture-secondary-credential", secondary.URL, "fixture-secondary-model"),
		},
		config.RouteConfig{Name: "fixture-fallback", Strategy: "fallback", Match: config.RouteMatch{Model: "fixture-virtual-model"}, Targets: []config.RouteTarget{{Provider: "primary", Model: "fixture-primary-model"}, {Provider: "secondary", Model: "fixture-secondary-model"}}},
		"fixture-virtual-model",
	)
	endpoint := httptest.NewServer(gateway.httpSrv.Handler)
	t.Cleanup(endpoint.Close)

	response := doLegacyGatewayRequest(t, endpoint.URL, fixtureClientCredential, `{"model":"fixture-virtual-model","messages":[{"role":"user","content":"fixture-input"}]}`)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || primaryCalls.Load() != 1 || secondaryCalls.Load() != 1 {
		t.Fatalf("current 400 fallback = status %d, primary %d, secondary %d", response.StatusCode, primaryCalls.Load(), secondaryCalls.Load())
	}
	t.Log("known gap: legacy fallback continues after an upstream 400; Task 37/40/42 own classified fallback eligibility")
}

func TestLegacyGatewayMapsUpstream429(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		w.Header().Set("Retry-After", "1")
		http.Error(w, `{"error":"fixture_rate_limited"}`, http.StatusTooManyRequests)
	}))
	t.Cleanup(upstream.Close)

	gateway := newLegacyGatewayFixture(t,
		[]config.ProviderConfig{legacyProviderConfig("primary", "fixture-primary-credential", upstream.URL, "fixture-upstream-model")},
		config.RouteConfig{Name: "fixture-route", Strategy: "round_robin", Match: config.RouteMatch{Model: "fixture-virtual-model"}, Targets: []config.RouteTarget{{Provider: "primary", Model: "fixture-upstream-model"}}},
		"fixture-virtual-model",
	)
	gateway.retryPolicy.MaxAttempts = 1
	endpoint := httptest.NewServer(gateway.httpSrv.Handler)
	t.Cleanup(endpoint.Close)

	response := doLegacyGatewayRequest(t, endpoint.URL, fixtureClientCredential, `{"model":"fixture-virtual-model","messages":[{"role":"user","content":"fixture-input"}]}`)
	defer response.Body.Close()
	if response.StatusCode != http.StatusTooManyRequests || upstreamCalls.Load() != 1 {
		t.Fatalf("429 mapping = status %d, upstream calls %d", response.StatusCode, upstreamCalls.Load())
	}
}

func TestLegacyGatewaySSEFirstEventAndDone(t *testing.T) {
	observed := make(chan legacyUpstreamObservation, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body provider.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode SSE request: %v", err)
			return
		}
		observed <- legacyUpstreamObservation{method: r.Method, path: r.URL.Path, authOK: r.Header.Get("Authorization") == "Bearer fixture-primary-credential", body: body}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"id\":\"fixture-stream\",\"object\":\"chat.completion.chunk\",\"model\":\"fixture-upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"fixture-first\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"id\":\"fixture-stream\",\"object\":\"chat.completion.chunk\",\"model\":\"fixture-upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"-second\"}}],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":2,\"total_tokens\":6}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(upstream.Close)

	gateway := newLegacyGatewayFixture(t,
		[]config.ProviderConfig{legacyProviderConfig("primary", "fixture-primary-credential", upstream.URL, "fixture-upstream-model")},
		config.RouteConfig{Name: "fixture-route", Strategy: "round_robin", Match: config.RouteMatch{Model: "fixture-virtual-model"}, Targets: []config.RouteTarget{{Provider: "primary", Model: "fixture-upstream-model"}}},
		"fixture-virtual-model",
	)
	endpoint := httptest.NewServer(gateway.httpSrv.Handler)
	t.Cleanup(endpoint.Close)

	response := doLegacyGatewayRequest(t, endpoint.URL, fixtureClientCredential, `{"model":"fixture-virtual-model","messages":[{"role":"user","content":"fixture-input"}],"stream":true}`)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("SSE response = status %d, content-type %q", response.StatusCode, response.Header.Get("Content-Type"))
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	events := strings.Split(strings.TrimSpace(string(body)), "\n\n")
	if len(events) != 3 || events[2] != "data: [DONE]" {
		t.Fatalf("SSE events = %q", events)
	}
	var first provider.StreamChunk
	if err := json.Unmarshal([]byte(strings.TrimPrefix(events[0], "data: ")), &first); err != nil {
		t.Fatal(err)
	}
	if first.Choices[0].Delta.Role != "assistant" || first.Choices[0].Delta.Content != "fixture-first" {
		t.Fatalf("first SSE event = %#v", first)
	}
	request := <-observed
	if !request.authOK || !request.body.Stream || request.body.Model != "fixture-upstream-model" {
		t.Fatalf("upstream SSE request = auth valid %v, stream %v, model %q", request.authOK, request.body.Stream, request.body.Model)
	}
}

func TestLegacyGatewayClientCancellationReachesStreamingUpstream(t *testing.T) {
	upstreamCanceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		_, _ = io.WriteString(w, "data: {\"id\":\"fixture-stream\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"fixture-first\"}}]}\n\n")
		flusher.Flush()
		<-r.Context().Done()
		close(upstreamCanceled)
	}))
	t.Cleanup(upstream.Close)

	gateway := newLegacyGatewayFixture(t,
		[]config.ProviderConfig{legacyProviderConfig("primary", "fixture-primary-credential", upstream.URL, "fixture-upstream-model")},
		config.RouteConfig{Name: "fixture-route", Strategy: "round_robin", Match: config.RouteMatch{Model: "fixture-virtual-model"}, Targets: []config.RouteTarget{{Provider: "primary", Model: "fixture-upstream-model"}}},
		"fixture-virtual-model",
	)
	endpoint := httptest.NewServer(gateway.httpSrv.Handler)
	t.Cleanup(endpoint.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL+"/v1/chat/completions", strings.NewReader(`{"model":"fixture-virtual-model","messages":[{"role":"user","content":"fixture-input"}],"stream":true}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+fixtureClientCredential)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(response.Body)
	firstLine, err := reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(firstLine, "data: ") {
		_ = response.Body.Close()
		t.Fatalf("first SSE line = %q, %v", firstLine, err)
	}
	cancel()
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case <-upstreamCanceled:
	case <-time.After(5 * time.Second):
		t.Fatal("client cancellation did not reach the streaming upstream")
	}
}

func newLegacyGatewayFixture(t *testing.T, providerConfigs []config.ProviderConfig, routeConfig config.RouteConfig, allowedModels string) *Server {
	t.Helper()
	providers := make(map[string]provider.LLMProvider, len(providerConfigs))
	for _, providerConfig := range providerConfigs {
		openAI, err := provider.NewOpenAI(providerConfig, testLogger.With("provider", providerConfig.Name))
		if err != nil {
			t.Fatal(err)
		}
		providers[providerConfig.Name] = openAI
	}
	routes, err := router.NewRouter([]config.RouteConfig{routeConfig})
	if err != nil {
		t.Fatal(err)
	}
	configuration := &config.Config{
		Server: config.ServerConfig{
			Port:         0,
			DBPath:       filepath.Join(t.TempDir(), "gateway.db"),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
		Auth: config.AuthConfig{
			Enabled: true,
			Keys: []config.KeyConfig{{
				Token:  fixtureClientCredential,
				Name:   "fixture-client",
				Role:   "user",
				Models: allowedModels,
			}},
		},
		Providers: providerConfigs,
		Routes:    []config.RouteConfig{routeConfig},
	}
	gateway, err := New(configuration, routes, providers, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if gateway.store != nil {
			if err := gateway.store.Close(); err != nil {
				t.Errorf("close gateway Store: %v", err)
			}
		}
	})
	return gateway
}

func legacyProviderConfig(name, credential, baseURL, model string) config.ProviderConfig {
	return config.ProviderConfig{Name: name, Type: "openai", APIKey: credential, BaseURL: baseURL, Models: []string{model}, Timeout: 5 * time.Second}
}

func doLegacyGatewayRequest(t *testing.T, baseURL, credential, body string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, baseURL+"/v1/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if credential != "" {
		request.Header.Set("Authorization", "Bearer "+credential)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
