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
	Quota     QuotaConfig       `yaml:"quota"`
	Cache     CacheConfig      `yaml:"cache"`
	Tracing   TracingConfig    `yaml:"tracing"`
	Filter    FilterConfig     `yaml:"filter"`
}

// FilterConfig controls PII / sensitive information filtering.
type FilterConfig struct {
	Enabled bool      `yaml:"enabled"`
	Mode    string    `yaml:"mode"`    // "mask" or "block"
	Rules   []string  `yaml:"rules"`   // enabled rule names: phone_cn, id_card_cn, email, credit_card, ipv4, api_key, cn_name
}

// TracingConfig configures OpenTelemetry tracing. All fields are optional; the
// zero value disables tracing entirely.
type TracingConfig struct {
	Enabled     bool    `yaml:"enabled"`
	Exporter    string  `yaml:"exporter"`     // "stdout" or "otlp"
	ServiceName string  `yaml:"service_name"`
	SampleRatio float64 `yaml:"sample_ratio"` // 0..1, 1 = sample everything
}

type KeyConfig struct {
	Token           string `yaml:"token"`
	Name            string `yaml:"name"`
	Role            string `yaml:"role"`
	DailyTokenLimit int64  `yaml:"daily_token_limit"`
	Models          string `yaml:"models"` // comma-separated allowlist, empty = all
}

type AuthConfig struct {
	Enabled bool        `yaml:"enabled"`
	Keys    []KeyConfig `yaml:"keys"`
}

type QuotaConfig struct {
	Enabled       bool `yaml:"enabled"`
	ResetHourUTC  int  `yaml:"reset_hour_utc"`
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
	Port           int             `yaml:"port"`
	DBPath         string          `yaml:"db_path"`
	MaxConcurrency int             `yaml:"max_concurrency"`
	QueueSize      int             `yaml:"queue_size"`
	QueueTimeout   time.Duration   `yaml:"queue_timeout"`
	Transport      TransportConfig `yaml:"transport"`
}

type TransportConfig struct {
	MaxConnsPerHost     int `yaml:"max_conns_per_host"`
	MaxIdleConnsPerHost int `yaml:"max_idle_conns_per_host"`
	MaxIdleConns        int `yaml:"max_idle_conns"`
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
