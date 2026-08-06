package main

import (
	"io"
	"log/slog"
	"testing"

	"ai-gateway/config"
)

func TestCreateProvidersSkipsDisabledNativeBootstrapDeclarations(t *testing.T) {
	cfg, err := config.Load("../../config/gateway.yaml")
	if err != nil {
		t.Fatal(err)
	}

	providers := createProviders(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if len(providers) != 1 {
		t.Fatalf("createProviders() count = %d, want only the invalid legacy provider", len(providers))
	}
	if _, ok := providers["legacy-invalid-example"]; !ok {
		t.Fatalf("createProviders() = %#v, want legacy-invalid-example", providers)
	}
	for _, name := range []string{"ark-bootstrap", "deepseek-bootstrap", "qwen-bootstrap"} {
		if _, ok := providers[name]; ok {
			t.Errorf("disabled native provider %q was constructed", name)
		}
	}
}
