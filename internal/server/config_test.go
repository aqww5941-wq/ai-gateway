package server

import (
	"testing"
	"time"

	"ai-gateway/config"
	"ai-gateway/internal/provider"
	"ai-gateway/internal/router"
)

func TestNewUsesConfiguredHTTPTimeouts(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:           8081,
			ReadTimeout:    17 * time.Second,
			WriteTimeout:   91 * time.Second,
			DBPath:         ":memory:",
			MaxConcurrency: 1,
		},
		Providers: []config.ProviderConfig{{
			Name: "mock", Type: "openai", APIKey: "not-a-real-key",
			BaseURL: "https://example.invalid/v1", Models: []string{"model-a"}, Timeout: time.Second,
		}},
		Routes: []config.RouteConfig{{
			Name: "default", Match: config.RouteMatch{Model: "model-a"}, Strategy: "round_robin",
			Targets: []config.RouteTarget{{Provider: "mock", Model: "model-a"}},
		}},
	}
	p, err := provider.NewOpenAI(cfg.Providers[0], testLogger)
	if err != nil {
		t.Fatal(err)
	}
	r, err := router.NewRouter(cfg.Routes)
	if err != nil {
		t.Fatal(err)
	}

	srv, err := New(cfg, r, map[string]provider.LLMProvider{"mock": p}, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	if srv.store != nil {
		t.Cleanup(func() { _ = srv.store.Close() })
	}
	if srv.httpSrv.ReadTimeout != cfg.Server.ReadTimeout || srv.httpSrv.WriteTimeout != cfg.Server.WriteTimeout {
		t.Fatalf("http timeouts = %s/%s, want %s/%s", srv.httpSrv.ReadTimeout, srv.httpSrv.WriteTimeout, cfg.Server.ReadTimeout, cfg.Server.WriteTimeout)
	}
}
