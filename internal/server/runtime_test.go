package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ai-gateway/config"
	"ai-gateway/internal/provider"
	"ai-gateway/internal/router"
)

func TestReloadPublishesCompleteImmutableSnapshotAndMonotonicRevision(t *testing.T) {
	oldUpstream := newReloadUpstream(t, "old")
	newUpstream := newReloadUpstream(t, "new")
	srv := newReloadTestServer(t, reloadTestConfig(oldUpstream.URL, "old-provider", "old-model"))

	oldSnapshot := srv.currentSnapshot()
	candidate := reloadTestConfig(newUpstream.URL, "new-provider", "new-model")
	if err := srv.Reload(candidate); err != nil {
		t.Fatal(err)
	}
	newSnapshot := srv.currentSnapshot()
	if newSnapshot == oldSnapshot || oldSnapshot.revision != 1 || newSnapshot.revision != 2 {
		t.Fatalf("snapshots/revisions = old(%p,%d) new(%p,%d), want distinct revisions 1 -> 2", oldSnapshot, oldSnapshot.revision, newSnapshot, newSnapshot.revision)
	}
	if _, ok := oldSnapshot.providers["old-provider"]; !ok {
		t.Fatalf("old snapshot lost old provider: %#v", oldSnapshot.providers)
	}
	if _, ok := newSnapshot.providers["new-provider"]; !ok {
		t.Fatalf("new snapshot missing new provider: %#v", newSnapshot.providers)
	}

	// Publishing clones the candidate; caller mutation cannot change the live
	// snapshot or its cross-references after acceptance.
	candidate.Providers[0].Name = "mutated-after-publish"
	candidate.Routes[0].Targets[0].Provider = "mutated-after-publish"
	if newSnapshot.config.Providers[0].Name != "new-provider" || newSnapshot.config.Routes[0].Targets[0].Provider != "new-provider" {
		t.Fatalf("published snapshot was mutated through candidate: %#v", newSnapshot.config)
	}

	assertReloadResponse(t, srv, "new")
	if err := srv.Reload(reloadTestConfig(oldUpstream.URL, "old-provider", "old-model")); err != nil {
		t.Fatal(err)
	}
	if got := srv.currentSnapshot().revision; got != 3 {
		t.Fatalf("revision = %d, want 3", got)
	}
}

func TestReloadNoOpDoesNotAdvanceRevision(t *testing.T) {
	upstream := newReloadUpstream(t, "same")
	cfg := reloadTestConfig(upstream.URL, "same-provider", "same-model")
	srv := newReloadTestServer(t, cfg)
	before := srv.currentSnapshot()

	if err := srv.Reload(config.Clone(cfg)); err != nil {
		t.Fatal(err)
	}
	if after := srv.currentSnapshot(); after != before || after.revision != 1 {
		t.Fatalf("no-op reload changed snapshot: before=%p/%d after=%p/%d", before, before.revision, after, after.revision)
	}
}

func TestReloadReusesOnlyUnaffectedRuntimeResources(t *testing.T) {
	firstUpstream := newReloadUpstream(t, "first")
	secondUpstream := newReloadUpstream(t, "second")
	cfg := reloadTestConfig(firstUpstream.URL, "provider", "model")
	srv := newReloadTestServer(t, cfg)
	initial := srv.currentSnapshot()

	routeOnly := config.Clone(cfg)
	routeOnly.Routes[0].Name = "renamed-route"
	if err := srv.Reload(routeOnly); err != nil {
		t.Fatal(err)
	}
	afterRoute := srv.currentSnapshot()
	if afterRoute.providers["provider"] != initial.providers["provider"] || afterRoute.breakers != initial.breakers {
		t.Fatal("route-only reload rebuilt unchanged provider resources")
	}
	if afterRoute.router == initial.router {
		t.Fatal("route-only reload reused changed router")
	}

	providerOnly := config.Clone(routeOnly)
	providerOnly.Providers[0].BaseURL = secondUpstream.URL
	if err := srv.Reload(providerOnly); err != nil {
		t.Fatal(err)
	}
	afterProvider := srv.currentSnapshot()
	if afterProvider.providers["provider"] == afterRoute.providers["provider"] || afterProvider.breakers == afterRoute.breakers {
		t.Fatal("provider reload reused provider or breaker state from the previous revision")
	}
	if afterProvider.router == afterRoute.router {
		t.Fatal("provider reload retained latency/router state for a changed provider")
	}
	assertReloadResponse(t, srv, "second")
}

func TestReloadFailureKeepsPreviousSnapshot(t *testing.T) {
	upstream := newReloadUpstream(t, "old")
	tests := []struct {
		name      string
		candidate func(*config.Config) *config.Config
		assertErr func(*testing.T, error)
	}{
		{
			name: "candidate validation",
			candidate: func(current *config.Config) *config.Config {
				candidate := config.Clone(current)
				candidate.Routes[0].Targets[0].Provider = "missing"
				return candidate
			},
			assertErr: func(t *testing.T, err error) {
				if err == nil || !strings.Contains(err.Error(), "validate reload candidate") {
					t.Fatalf("Reload() error = %v, want validation stage", err)
				}
			},
		},
		{
			name: "restart required",
			candidate: func(current *config.Config) *config.Config {
				candidate := config.Clone(current)
				candidate.Server.Port++
				return candidate
			},
			assertErr: func(t *testing.T, err error) {
				sections, ok := config.RestartRequiredSections(err)
				if !ok || len(sections) != 1 || sections[0] != "server" {
					t.Fatalf("RestartRequiredSections() = %v, %v, want [server], true (error: %v)", sections, ok, err)
				}
			},
		},
		{
			name: "snapshot build",
			candidate: func(current *config.Config) *config.Config {
				candidate := config.Clone(current)
				enabled := false
				candidate.Providers = []config.ProviderConfig{{
					Name:       "qwen-bootstrap",
					Kind:       "qwen",
					Enabled:    &enabled,
					Credential: config.ProviderCredentialRef{Env: "DASHSCOPE_API_KEY"},
					Evidence:   config.ProviderEvidenceConfig{Status: "unverified"},
					Qwen: &config.QwenProviderConfig{
						BaseURL:         "https://invalid-example-workspace-id.cn-beijing.maas.aliyuncs.com/compatible-mode/v1",
						Region:          "cn-beijing",
						ProtocolVersion: "chat-completions-v1",
						WorkspaceID:     "invalid-example-workspace-id",
					},
					Models:  []string{"qwen3.7-flash"},
					Timeout: 30 * time.Second,
				}}
				candidate.Routes = nil
				return candidate
			},
			assertErr: func(t *testing.T, err error) {
				if err == nil || !strings.Contains(err.Error(), "build runtime snapshot") {
					t.Fatalf("Reload() error = %v, want snapshot build stage", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newReloadTestServer(t, reloadTestConfig(upstream.URL, "old-provider", "old-model"))
			before := srv.currentSnapshot()
			err := srv.Reload(tt.candidate(before.config))
			tt.assertErr(t, err)
			after := srv.currentSnapshot()
			if after != before || after.revision != 1 {
				t.Fatalf("rejected reload published snapshot: before=%p/%d after=%p/%d", before, before.revision, after, after.revision)
			}
			assertReloadResponse(t, srv, "old")
		})
	}
}

func TestConcurrentRequestsObserveOneCompleteReloadRevision(t *testing.T) {
	oldUpstream := newReloadUpstream(t, "old")
	newUpstream := newReloadUpstream(t, "new")
	oldConfig := reloadTestConfig(oldUpstream.URL, "old-provider", "old-model")
	newConfig := reloadTestConfig(newUpstream.URL, "new-provider", "new-model")
	srv := newReloadTestServer(t, oldConfig)

	const workers = 8
	const requestsPerWorker = 40
	errors := make(chan error, workers*requestsPerWorker)
	var workersDone sync.WaitGroup
	workersDone.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer workersDone.Done()
			for requestIndex := 0; requestIndex < requestsPerWorker; requestIndex++ {
				response, err := reloadResponse(srv)
				if err != nil {
					errors <- err
					continue
				}
				if response != "old" && response != "new" {
					errors <- fmt.Errorf("response = %q, want old or new", response)
				}
			}
		}()
	}

	for reloadIndex := 0; reloadIndex < 40; reloadIndex++ {
		candidate := oldConfig
		if reloadIndex%2 == 0 {
			candidate = newConfig
		}
		if err := srv.Reload(candidate); err != nil {
			t.Fatalf("Reload() iteration %d: %v", reloadIndex, err)
		}
	}
	workersDone.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func reloadTestConfig(baseURL, providerName, upstreamModel string) *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Port: 8081, ReadTimeout: 30 * time.Second, WriteTimeout: 120 * time.Second,
			DBPath: ":memory:", MaxConcurrency: 16, QueueSize: 0, QueueTimeout: 10 * time.Second,
			Transport: config.TransportConfig{MaxConnsPerHost: 16, MaxIdleConnsPerHost: 8, MaxIdleConns: 16},
		},
		Providers: []config.ProviderConfig{{
			Name: providerName, Type: "openai", APIKey: "invalid-example-key", BaseURL: baseURL,
			Models: []string{upstreamModel}, Timeout: 5 * time.Second,
		}},
		Routes: []config.RouteConfig{{
			Name: "virtual-route", Match: config.RouteMatch{Model: "virtual-model"}, Strategy: "round_robin",
			Targets: []config.RouteTarget{{Provider: providerName, Model: upstreamModel}},
		}},
		RateLimit: config.RateLimitConfig{PerKey: 60, PerModel: 100},
		Cache: config.CacheConfig{
			Backend: "memory", TTL: "1h", Strategy: "exact", MaxSize: 1000, Threshold: 0.85, RedisAddr: "localhost:6379",
		},
		Tracing: config.TracingConfig{Exporter: "stdout", ServiceName: "ai-gateway", SampleRatio: 1},
		Filter:  config.FilterConfig{Mode: "mask"},
	}
}

func newReloadTestServer(t *testing.T, cfg *config.Config) *Server {
	t.Helper()
	if err := config.Validate(cfg); err != nil {
		t.Fatal(err)
	}
	instance, err := provider.NewOpenAI(cfg.Providers[0], testLogger)
	if err != nil {
		t.Fatal(err)
	}
	runtimeRouter, err := router.NewRouter(cfg.Routes)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(cfg, runtimeRouter, map[string]provider.LLMProvider{cfg.Providers[0].Name: instance}, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	if srv.store != nil {
		t.Cleanup(func() { _ = srv.store.Close() })
	}
	return srv
}

func newReloadUpstream(t *testing.T, identity string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(&provider.ChatResponse{
			ID: "reload-" + identity, Object: "chat.completion", Model: identity,
			Choices: []provider.Choice{{Index: 0, Message: provider.Message{Role: "assistant", Content: identity}, FinishReason: "stop"}},
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func assertReloadResponse(t *testing.T, srv *Server, want string) {
	t.Helper()
	got, err := reloadResponse(srv)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("response = %q, want %q", got, want)
	}
}

func reloadResponse(srv *Server) (string, error) {
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"virtual-model","messages":[{"role":"user","content":"reload"}]}`))
	recorder := httptest.NewRecorder()
	srv.handleChatCompletion(recorder, request)
	if recorder.Code != http.StatusOK {
		return "", fmt.Errorf("status = %d: %s", recorder.Code, recorder.Body.String())
	}
	var response provider.ChatResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		return "", err
	}
	if len(response.Choices) != 1 {
		return "", fmt.Errorf("choices = %d, want 1", len(response.Choices))
	}
	return response.Choices[0].Message.Content, nil
}
