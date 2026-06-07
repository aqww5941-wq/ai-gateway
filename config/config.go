package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig     `yaml:"server"`
	Auth      AuthConfig       `yaml:"auth"`
	Providers []ProviderConfig `yaml:"providers"`
	Routes    []RouteConfig    `yaml:"routes"`
	RateLimit RateLimitConfig  `yaml:"rate_limit"`
	Cache     CacheConfig      `yaml:"cache"`
	Tracing   TracingConfig    `yaml:"tracing"`
}

// TracingConfig configures OpenTelemetry tracing. All fields are optional; the
// zero value disables tracing entirely.
type TracingConfig struct {
	Enabled     bool    `yaml:"enabled"`
	Exporter    string  `yaml:"exporter"`     // "stdout" or "otlp"
	ServiceName string  `yaml:"service_name"`
	SampleRatio float64 `yaml:"sample_ratio"` // 0..1, 1 = sample everything
}

type AuthConfig struct {
	Enabled bool     `yaml:"enabled"`
	Keys    []string `yaml:"keys"`
}

type RateLimitConfig struct {
	Enabled  bool `yaml:"enabled"`
	PerKey   int  `yaml:"per_key"`
	PerModel int  `yaml:"per_model"`
}

type CacheConfig struct {
	Enabled    bool    `yaml:"enabled"`
	Backend    string  `yaml:"backend"`
	TTL        string  `yaml:"ttl"`
	Strategy   string  `yaml:"strategy"`
	MaxSize    int     `yaml:"max_size"`
	Threshold  float64 `yaml:"threshold"`
	RedisAddr  string  `yaml:"redis_addr"`
	RedisPass  string  `yaml:"redis_pass"`
	RedisDB    int     `yaml:"redis_db"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type ProviderConfig struct {
	Name    string        `yaml:"name"`
	Type    string        `yaml:"type"`
	APIKey  string        `yaml:"api_key"`
	BaseURL string        `yaml:"base_url"`
	Models  []string      `yaml:"models"`
	Timeout time.Duration `yaml:"timeout"`
}

type RouteConfig struct {
	Name          string               `yaml:"name"`
	Match         RouteMatch           `yaml:"match"`
	Strategy      string               `yaml:"strategy"`
	Targets       []RouteTarget        `yaml:"targets"`
	SemanticRules []SemanticRuleConfig `yaml:"semantic_rules"`
}

type RouteMatch struct {
	Model string `yaml:"model"`
}

type RouteTarget struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	Weight   int    `yaml:"weight"`
}

type SemanticRuleConfig struct {
	Complexity string      `yaml:"complexity"`
	Target     RouteTarget `yaml:"target"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}
