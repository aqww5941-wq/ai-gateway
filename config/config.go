package config

import "time"

type Config struct {
	Server    ServerConfig     `yaml:"server"`
	Auth      AuthConfig       `yaml:"auth"`
	Providers []ProviderConfig `yaml:"providers"`
	Routes    []RouteConfig    `yaml:"routes"`
	RateLimit RateLimitConfig  `yaml:"rate_limit"`
	Quota     QuotaConfig      `yaml:"quota"`
	Cache     CacheConfig      `yaml:"cache"`
	Tracing   TracingConfig    `yaml:"tracing"`
	Filter    FilterConfig     `yaml:"filter"`
}

// FilterConfig controls PII / sensitive information filtering.
type FilterConfig struct {
	Enabled bool     `yaml:"enabled"`
	Mode    string   `yaml:"mode"`  // "mask" or "block"
	Rules   []string `yaml:"rules"` // enabled rule names: phone_cn, id_card_cn, email, credit_card, ipv4, api_key, cn_name
}

// TracingConfig configures OpenTelemetry tracing. All fields are optional; the
// zero value disables tracing entirely.
type TracingConfig struct {
	Enabled     bool    `yaml:"enabled"`
	Exporter    string  `yaml:"exporter"` // "stdout" or "otlp"
	ServiceName string  `yaml:"service_name"`
	SampleRatio float64 `yaml:"sample_ratio"` // (0,1], 1 = sample everything
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
	Enabled      bool `yaml:"enabled"`
	ResetHourUTC int  `yaml:"reset_hour_utc"`
}

type RateLimitConfig struct {
	Enabled  bool `yaml:"enabled"`
	PerKey   int  `yaml:"per_key"`
	PerModel int  `yaml:"per_model"`
}

type CacheConfig struct {
	Enabled   bool    `yaml:"enabled"`
	Backend   string  `yaml:"backend"`
	TTL       string  `yaml:"ttl"`
	Strategy  string  `yaml:"strategy"`
	MaxSize   int     `yaml:"max_size"`
	Threshold float64 `yaml:"threshold"`
	RedisAddr string  `yaml:"redis_addr"`
	RedisPass string  `yaml:"redis_pass"`
	RedisDB   int     `yaml:"redis_db"`
}

type ServerConfig struct {
	Port           int             `yaml:"port"`
	ReadTimeout    time.Duration   `yaml:"read_timeout"`
	WriteTimeout   time.Duration   `yaml:"write_timeout"`
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
	Name       string                  `yaml:"name"`
	Kind       string                  `yaml:"kind,omitempty"`
	Enabled    *bool                   `yaml:"enabled,omitempty"`
	Credential ProviderCredentialRef   `yaml:"credential,omitempty"`
	Evidence   ProviderEvidenceConfig  `yaml:"evidence,omitempty"`
	Ark        *ArkProviderConfig      `yaml:"ark,omitempty"`
	DeepSeek   *DeepSeekProviderConfig `yaml:"deepseek,omitempty"`
	Qwen       *QwenProviderConfig     `yaml:"qwen,omitempty"`
	Models     []string                `yaml:"models"`
	Timeout    time.Duration           `yaml:"timeout"`

	// Type, APIKey, and BaseURL preserve the current OpenAI/Claude adapter
	// contract during the strangler migration. New native providers must use
	// Kind plus their vendor-specific schema and may not populate these fields.
	Type    string `yaml:"type,omitempty"`
	APIKey  string `yaml:"api_key,omitempty"`
	BaseURL string `yaml:"base_url,omitempty"`
}

// RuntimeEnabled distinguishes legacy adapters (which predate an enabled
// field) from native bootstrap declarations. Native declarations are not
// runnable until their dedicated adapters are implemented.
func (p ProviderConfig) RuntimeEnabled() bool {
	if p.Kind == "" {
		return true
	}
	return p.Enabled != nil && *p.Enabled
}

// ProviderCredentialRef identifies a secret source without copying the
// credential value into bootstrap configuration.
type ProviderCredentialRef struct {
	Env string `yaml:"env"`
}

type ProviderEvidenceConfig struct {
	Status string `yaml:"status"`
}

type ArkProviderConfig struct {
	BaseURL         string `yaml:"base_url"`
	Region          string `yaml:"region"`
	ProtocolVersion string `yaml:"protocol_version"`
	EndpointID      string `yaml:"endpoint_id"`
}

type DeepSeekProviderConfig struct {
	BaseURL         string `yaml:"base_url"`
	Region          string `yaml:"region"`
	ProtocolVersion string `yaml:"protocol_version"`
	Endpoint        string `yaml:"endpoint"`
}

type QwenProviderConfig struct {
	BaseURL         string `yaml:"base_url"`
	Region          string `yaml:"region"`
	ProtocolVersion string `yaml:"protocol_version"`
	WorkspaceID     string `yaml:"workspace_id"`
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
