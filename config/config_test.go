package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const minimalConfigYAML = `providers:
  - name: primary
    type: openai
    api_key: not-a-real-key
    base_url: https://example.invalid/v1
    models: [model-a]
routes:
  - name: default
    match:
      model: model-a
    strategy: round_robin
    targets:
      - provider: primary
        model: model-a
`

func TestLoadAppliesDeterministicDefaults(t *testing.T) {
	cfg, err := loadTestConfig(t, minimalConfigYAML)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server.Port != 8081 || cfg.Server.ReadTimeout != 30*time.Second || cfg.Server.WriteTimeout != 120*time.Second {
		t.Fatalf("server defaults = port %d, read %s, write %s", cfg.Server.Port, cfg.Server.ReadTimeout, cfg.Server.WriteTimeout)
	}
	if cfg.Server.DBPath != "data/gateway.db" || cfg.Server.MaxConcurrency != 500 || cfg.Server.QueueSize != 200 || cfg.Server.QueueTimeout != 10*time.Second {
		t.Fatalf("server resource defaults = %#v", cfg.Server)
	}
	if got := cfg.Server.Transport; got != (TransportConfig{MaxConnsPerHost: 100, MaxIdleConnsPerHost: 50, MaxIdleConns: 200}) {
		t.Fatalf("transport defaults = %#v", got)
	}
	if cfg.Providers[0].Timeout != 30*time.Second {
		t.Fatalf("provider timeout = %s, want 30s", cfg.Providers[0].Timeout)
	}
	if cfg.RateLimit.PerKey != 60 || cfg.RateLimit.PerModel != 100 {
		t.Fatalf("rate limit defaults = %#v", cfg.RateLimit)
	}
	if cfg.Cache.Backend != "memory" || cfg.Cache.TTL != "1h" || cfg.Cache.Strategy != "exact" || cfg.Cache.MaxSize != 1000 || cfg.Cache.Threshold != 0.85 || cfg.Cache.RedisAddr != "localhost:6379" {
		t.Fatalf("cache defaults = %#v", cfg.Cache)
	}
	if cfg.Tracing.Exporter != "stdout" || cfg.Tracing.ServiceName != "ai-gateway" || cfg.Tracing.SampleRatio != 1 {
		t.Fatalf("tracing defaults = %#v", cfg.Tracing)
	}
	if cfg.Filter.Mode != "mask" {
		t.Fatalf("filter mode = %q, want mask", cfg.Filter.Mode)
	}
}

func TestLoadCurrentExample(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "task3-deepseek-key")
	t.Setenv("SILICONFLOW_API_KEY", "task3-siliconflow-key")
	t.Setenv("DOUBAO_API_KEY", "task3-doubao-key")

	cfg, err := Load("gateway.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.ReadTimeout != 30*time.Second || cfg.Server.WriteTimeout != 120*time.Second {
		t.Fatalf("example timeouts = %s/%s", cfg.Server.ReadTimeout, cfg.Server.WriteTimeout)
	}
}

func TestLoadRejectsMissingEnvironmentVariablesInSortedOrder(t *testing.T) {
	for _, name := range []string{"TASK3_MISSING_A", "TASK3_MISSING_B"} {
		value, existed := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(name, value)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}

	_, err := loadTestConfig(t, strings.ReplaceAll(minimalConfigYAML, "not-a-real-key", "${TASK3_MISSING_B}-${TASK3_MISSING_A}"))
	assertConfigError(t, err, ErrorKindEnvironment, "environment", "missing variables: TASK3_MISSING_A, TASK3_MISSING_B")
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	_, err := loadTestConfig(t, "server:\n  mystery_timeout: 5s\n"+minimalConfigYAML)
	if err == nil {
		t.Fatal("Load() error = nil, want unknown field error")
	}
	kind, ok := ErrorKindOf(err)
	if !ok || kind != ErrorKindParse {
		t.Fatalf("ErrorKindOf() = %q, %v, want %q, true", kind, ok, ErrorKindParse)
	}
	if !strings.Contains(err.Error(), "field mystery_timeout not found in type config.ServerConfig") {
		t.Fatalf("Load() error = %q, want located unknown field", err)
	}
}

func TestLoadReadErrorRetainsCause(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	_, err := Load(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load() error = %v, want os.ErrNotExist cause", err)
	}
	kind, ok := ErrorKindOf(err)
	if !ok || kind != ErrorKindRead {
		t.Fatalf("ErrorKindOf() = %q, %v, want %q, true", kind, ok, ErrorKindRead)
	}
}

func TestLoadRejectsMalformedDuration(t *testing.T) {
	_, err := loadTestConfig(t, "server:\n  read_timeout: immediately\n"+minimalConfigYAML)
	if err == nil {
		t.Fatal("Load() error = nil, want duration parse error")
	}
	kind, ok := ErrorKindOf(err)
	if !ok || kind != ErrorKindParse {
		t.Fatalf("ErrorKindOf() = %q, %v, want %q, true", kind, ok, ErrorKindParse)
	}
	if !strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "time.Duration") {
		t.Fatalf("Load() error = %q, want line and duration type", err)
	}
}

func TestLoadRejectsMultipleYAMLDocuments(t *testing.T) {
	_, err := loadTestConfig(t, minimalConfigYAML+"---\n{}\n")
	assertConfigError(t, err, ErrorKindParse, "document", "multiple YAML documents are not allowed")
}

func TestLoadDoesNotReplaceExplicitZeroWithDefaults(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		path    string
		problem string
	}{
		{name: "port", prefix: "server:\n  port: 0\n", path: "server.port", problem: "must be between 1 and 65535"},
		{name: "read timeout", prefix: "server:\n  read_timeout: 0s\n", path: "server.read_timeout", problem: "must be greater than 0"},
		{name: "sample ratio", prefix: "tracing:\n  sample_ratio: 0\n", path: "tracing.sample_ratio", problem: "must be greater than 0 and less than or equal to 1"},
		{name: "cache threshold", prefix: "cache:\n  threshold: 0\n", path: "cache.threshold", problem: "must be greater than 0 and less than or equal to 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadTestConfig(t, tt.prefix+minimalConfigYAML)
			assertConfigError(t, err, ErrorKindValidation, tt.path, tt.problem)
		})
	}
}

func TestLoadDistinguishesMissingAndExplicitZeroProviderTimeout(t *testing.T) {
	cfg, err := loadTestConfig(t, minimalConfigYAML)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Providers[0].Timeout != 30*time.Second {
		t.Fatalf("missing timeout default = %s, want 30s", cfg.Providers[0].Timeout)
	}

	withZero := strings.Replace(minimalConfigYAML, "    models: [model-a]", "    models: [model-a]\n    timeout: 0s", 1)
	_, err = loadTestConfig(t, withZero)
	assertConfigError(t, err, ErrorKindValidation, "providers[0].timeout", "must be greater than 0")

	withNull := strings.Replace(minimalConfigYAML, "    models: [model-a]", "    models: [model-a]\n    timeout: null", 1)
	_, err = loadTestConfig(t, withNull)
	assertConfigError(t, err, ErrorKindValidation, "providers[0].timeout", "must be greater than 0")
}

func TestValidateRejectsInvalidAndDanglingConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		path    string
		problem string
	}{
		{
			name:   "port",
			mutate: func(cfg *Config) { cfg.Server.Port = 65536 },
			path:   "server.port", problem: "must be between 1 and 65535",
		},
		{
			name:   "duration",
			mutate: func(cfg *Config) { cfg.Server.ReadTimeout = -time.Second },
			path:   "server.read_timeout", problem: "must be greater than 0",
		},
		{
			name:   "ratio",
			mutate: func(cfg *Config) { cfg.Tracing.SampleRatio = 1.01 },
			path:   "tracing.sample_ratio", problem: "must be greater than 0 and less than or equal to 1",
		},
		{
			name:   "duplicate provider",
			mutate: func(cfg *Config) { cfg.Providers = append(cfg.Providers, cfg.Providers[0]) },
			path:   "providers[1].name", problem: `duplicates provider name "primary"`,
		},
		{
			name:   "duplicate route name",
			mutate: func(cfg *Config) { cfg.Routes = append(cfg.Routes, cfg.Routes[0]) },
			path:   "routes[1].name", problem: `duplicates route name "default"`,
		},
		{
			name: "duplicate route model",
			mutate: func(cfg *Config) {
				duplicate := cfg.Routes[0]
				duplicate.Name = "another"
				cfg.Routes = append(cfg.Routes, duplicate)
			},
			path: "routes[1].match.model", problem: `duplicates route model "model-a"`,
		},
		{
			name:   "unknown provider reference",
			mutate: func(cfg *Config) { cfg.Routes[0].Targets[0].Provider = "missing" },
			path:   "routes[0].targets[0].provider", problem: `references unknown provider "missing"`,
		},
		{
			name:   "unknown provider model reference",
			mutate: func(cfg *Config) { cfg.Routes[0].Targets[0].Model = "missing-model" },
			path:   "routes[0].targets[0].model", problem: `references model "missing-model" not declared by provider "primary"`,
		},
		{
			name: "unknown route reference",
			mutate: func(cfg *Config) {
				cfg.Auth.Keys = []KeyConfig{{Token: "not-a-real-token", Name: "user", Role: "user", Models: "missing-route"}}
			},
			path: "auth.keys[0].models", problem: `references unknown route model "missing-route"`,
		},
		{
			name:   "invalid route strategy",
			mutate: func(cfg *Config) { cfg.Routes[0].Strategy = "random" },
			path:   "routes[0].strategy", problem: "must be one of: fallback, latency, round_robin, semantic, weighted",
		},
		{
			name: "incomplete semantic route",
			mutate: func(cfg *Config) {
				cfg.Routes[0].Strategy = "semantic"
				cfg.Routes[0].Targets = nil
				cfg.Routes[0].SemanticRules = []SemanticRuleConfig{{Complexity: "simple", Target: RouteTarget{Provider: "primary", Model: "model-a"}}}
			},
			path: "routes[0].semantic_rules", problem: `missing "complex" complexity rule`,
		},
		{
			name:   "invalid cache duration",
			mutate: func(cfg *Config) { cfg.Cache.TTL = "0s" },
			path:   "cache.ttl", problem: "must be greater than 0",
		},
		{
			name:   "invalid filter rule",
			mutate: func(cfg *Config) { cfg.Filter.Rules = []string{"unknown"} },
			path:   "filter.rules[0]", problem: `unknown filter rule "unknown"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validTestConfig(t)
			tt.mutate(cfg)
			err := Validate(cfg)
			assertConfigError(t, err, ErrorKindValidation, tt.path, tt.problem)
		})
	}
}

func TestErrorKindOfRejectsNonConfigError(t *testing.T) {
	if kind, ok := ErrorKindOf(errors.New("other")); ok || kind != "" {
		t.Fatalf("ErrorKindOf() = %q, %v, want empty, false", kind, ok)
	}
}

func validTestConfig(t *testing.T) *Config {
	t.Helper()
	cfg, err := decodeStrict(minimalConfigYAML)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func loadTestConfig(t *testing.T, contents string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

func assertConfigError(t *testing.T, err error, kind ErrorKind, path, problem string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s error", kind)
	}
	var configErr *ConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("error = %T %v, want *ConfigError", err, err)
	}
	if configErr.Kind != kind || configErr.Path != path || configErr.Problem != problem {
		t.Fatalf("ConfigError = %#v, want kind=%q path=%q problem=%q", configErr, kind, path, problem)
	}
}
