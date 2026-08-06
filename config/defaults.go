package config

import "time"

const (
	defaultServerPort            = 8081
	defaultServerReadTimeout     = 30 * time.Second
	defaultServerWriteTimeout    = 120 * time.Second
	defaultServerDBPath          = "data/gateway.db"
	defaultMaxConcurrency        = 500
	defaultQueueSize             = 200
	defaultQueueTimeout          = 10 * time.Second
	defaultMaxConnsPerHost       = 100
	defaultMaxIdleConnsPerHost   = 50
	defaultMaxIdleConns          = 200
	defaultOpenAIProviderTimeout = 30 * time.Second
	defaultClaudeProviderTimeout = 60 * time.Second
	defaultRateLimitPerKey       = 60
	defaultRateLimitPerModel     = 100
	defaultCacheTTL              = "1h"
	defaultCacheMaxSize          = 1000
	defaultCacheThreshold        = 0.85
	defaultRedisAddr             = "localhost:6379"
	defaultTracingServiceName    = "ai-gateway"
	defaultTracingSampleRatio    = 1.0
)

func configWithDefaults() Config {
	return Config{
		Server: ServerConfig{
			Port:           defaultServerPort,
			ReadTimeout:    defaultServerReadTimeout,
			WriteTimeout:   defaultServerWriteTimeout,
			DBPath:         defaultServerDBPath,
			MaxConcurrency: defaultMaxConcurrency,
			QueueSize:      defaultQueueSize,
			QueueTimeout:   defaultQueueTimeout,
			Transport: TransportConfig{
				MaxConnsPerHost:     defaultMaxConnsPerHost,
				MaxIdleConnsPerHost: defaultMaxIdleConnsPerHost,
				MaxIdleConns:        defaultMaxIdleConns,
			},
		},
		RateLimit: RateLimitConfig{
			PerKey:   defaultRateLimitPerKey,
			PerModel: defaultRateLimitPerModel,
		},
		Cache: CacheConfig{
			Backend:   "memory",
			TTL:       defaultCacheTTL,
			Strategy:  "exact",
			MaxSize:   defaultCacheMaxSize,
			Threshold: defaultCacheThreshold,
			RedisAddr: defaultRedisAddr,
		},
		Tracing: TracingConfig{
			Exporter:    "stdout",
			ServiceName: defaultTracingServiceName,
			SampleRatio: defaultTracingSampleRatio,
		},
		Filter: FilterConfig{Mode: "mask"},
	}
}

func applyProviderTimeoutDefaults(cfg *Config, timeoutPresent []bool) {
	for i := range cfg.Providers {
		if i < len(timeoutPresent) && timeoutPresent[i] {
			continue
		}
		if cfg.Providers[i].Type == "claude" {
			cfg.Providers[i].Timeout = defaultClaudeProviderTimeout
		} else {
			cfg.Providers[i].Timeout = defaultOpenAIProviderTimeout
		}
	}
}
