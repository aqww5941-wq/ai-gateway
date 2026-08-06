package config

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

var validFilterRules = map[string]struct{}{
	"api_key":     {},
	"cn_name":     {},
	"credit_card": {},
	"email":       {},
	"id_card_cn":  {},
	"ipv4":        {},
	"phone_cn":    {},
}

// Validate checks the complete current bootstrap contract in deterministic
// field order. It never mutates cfg and never includes secret values in errors.
func Validate(cfg *Config) error {
	if cfg == nil {
		return validationError("config", "must not be null")
	}
	if err := validateServer(cfg.Server); err != nil {
		return err
	}
	providers, err := validateProviders(cfg.Providers)
	if err != nil {
		return err
	}
	routeModels, err := validateRoutes(cfg.Routes, providers)
	if err != nil {
		return err
	}
	if err := validateAuth(cfg.Auth, routeModels); err != nil {
		return err
	}
	if err := validateRateLimit(cfg.RateLimit); err != nil {
		return err
	}
	if cfg.Quota.ResetHourUTC < 0 || cfg.Quota.ResetHourUTC > 23 {
		return validationError("quota.reset_hour_utc", "must be between 0 and 23")
	}
	if err := validateCache(cfg.Cache); err != nil {
		return err
	}
	if err := validateTracing(cfg.Tracing); err != nil {
		return err
	}
	return validateFilter(cfg.Filter)
}

func validateServer(cfg ServerConfig) error {
	if cfg.Port < 1 || cfg.Port > 65535 {
		return validationError("server.port", "must be between 1 and 65535")
	}
	if cfg.ReadTimeout <= 0 {
		return validationError("server.read_timeout", "must be greater than 0")
	}
	if cfg.WriteTimeout <= 0 {
		return validationError("server.write_timeout", "must be greater than 0")
	}
	if strings.TrimSpace(cfg.DBPath) == "" {
		return validationError("server.db_path", "must not be empty")
	}
	if cfg.MaxConcurrency <= 0 {
		return validationError("server.max_concurrency", "must be greater than 0")
	}
	if cfg.QueueSize < 0 {
		return validationError("server.queue_size", "must be greater than or equal to 0")
	}
	if cfg.QueueSize > 0 && cfg.QueueTimeout <= 0 {
		return validationError("server.queue_timeout", "must be greater than 0 when queueing is enabled")
	}
	if cfg.Transport.MaxConnsPerHost <= 0 {
		return validationError("server.transport.max_conns_per_host", "must be greater than 0")
	}
	if cfg.Transport.MaxIdleConnsPerHost <= 0 {
		return validationError("server.transport.max_idle_conns_per_host", "must be greater than 0")
	}
	if cfg.Transport.MaxIdleConns <= 0 {
		return validationError("server.transport.max_idle_conns", "must be greater than 0")
	}
	if cfg.Transport.MaxIdleConnsPerHost > cfg.Transport.MaxIdleConns {
		return validationError("server.transport.max_idle_conns_per_host", "must not exceed server.transport.max_idle_conns")
	}
	return nil
}

func validateProviders(configs []ProviderConfig) (map[string]map[string]struct{}, error) {
	if len(configs) == 0 {
		return nil, validationError("providers", "must contain at least one provider")
	}
	providers := make(map[string]map[string]struct{}, len(configs))
	for i, cfg := range configs {
		path := "providers[" + strconv.Itoa(i) + "]"
		if err := validateName(path+".name", cfg.Name); err != nil {
			return nil, err
		}
		if _, exists := providers[cfg.Name]; exists {
			return nil, validationError(path+".name", "duplicates provider name %q", cfg.Name)
		}
		if cfg.Type != "openai" && cfg.Type != "claude" {
			return nil, validationError(path+".type", "must be one of: openai, claude")
		}
		if cfg.APIKey == "" {
			return nil, validationError(path+".api_key", "must not be empty")
		}
		if cfg.Type == "openai" && strings.TrimSpace(cfg.BaseURL) == "" {
			return nil, validationError(path+".base_url", "must not be empty")
		}
		if cfg.BaseURL != "" {
			if err := validateBaseURL(path+".base_url", cfg.BaseURL); err != nil {
				return nil, err
			}
		}
		if cfg.Timeout <= 0 {
			return nil, validationError(path+".timeout", "must be greater than 0")
		}
		if len(cfg.Models) == 0 {
			return nil, validationError(path+".models", "must contain at least one model")
		}
		models := make(map[string]struct{}, len(cfg.Models))
		for modelIndex, model := range cfg.Models {
			modelPath := path + ".models[" + strconv.Itoa(modelIndex) + "]"
			if err := validateName(modelPath, model); err != nil {
				return nil, err
			}
			if _, exists := models[model]; exists {
				return nil, validationError(modelPath, "duplicates model %q", model)
			}
			models[model] = struct{}{}
		}
		providers[cfg.Name] = models
	}
	return providers, nil
}

func validateRoutes(configs []RouteConfig, providers map[string]map[string]struct{}) (map[string]struct{}, error) {
	if len(configs) == 0 {
		return nil, validationError("routes", "must contain at least one route")
	}
	names := make(map[string]struct{}, len(configs))
	models := make(map[string]struct{}, len(configs))
	for i, cfg := range configs {
		path := "routes[" + strconv.Itoa(i) + "]"
		if err := validateName(path+".name", cfg.Name); err != nil {
			return nil, err
		}
		if _, exists := names[cfg.Name]; exists {
			return nil, validationError(path+".name", "duplicates route name %q", cfg.Name)
		}
		names[cfg.Name] = struct{}{}
		if err := validateName(path+".match.model", cfg.Match.Model); err != nil {
			return nil, err
		}
		if _, exists := models[cfg.Match.Model]; exists {
			return nil, validationError(path+".match.model", "duplicates route model %q", cfg.Match.Model)
		}
		models[cfg.Match.Model] = struct{}{}

		switch cfg.Strategy {
		case "round_robin", "weighted", "fallback", "latency":
			if len(cfg.Targets) == 0 {
				return nil, validationError(path+".targets", "must contain at least one target for strategy %q", cfg.Strategy)
			}
			if len(cfg.SemanticRules) != 0 {
				return nil, validationError(path+".semantic_rules", "must be empty for strategy %q", cfg.Strategy)
			}
			for targetIndex, target := range cfg.Targets {
				targetPath := path + ".targets[" + strconv.Itoa(targetIndex) + "]"
				if err := validateTarget(targetPath, target, providers); err != nil {
					return nil, err
				}
				if (cfg.Strategy == "weighted" || cfg.Strategy == "latency") && target.Weight <= 0 {
					return nil, validationError(targetPath+".weight", "must be greater than 0 for strategy %q", cfg.Strategy)
				}
			}
		case "semantic":
			if len(cfg.Targets) != 0 {
				return nil, validationError(path+".targets", "must be empty for strategy %q", cfg.Strategy)
			}
			if err := validateSemanticRules(path, cfg.SemanticRules, providers); err != nil {
				return nil, err
			}
		default:
			return nil, validationError(path+".strategy", "must be one of: fallback, latency, round_robin, semantic, weighted")
		}
	}
	return models, nil
}

func validateSemanticRules(path string, rules []SemanticRuleConfig, providers map[string]map[string]struct{}) error {
	if len(rules) == 0 {
		return validationError(path+".semantic_rules", "must contain simple and complex rules")
	}
	seen := make(map[string]struct{}, len(rules))
	for i, rule := range rules {
		rulePath := path + ".semantic_rules[" + strconv.Itoa(i) + "]"
		if rule.Complexity != "simple" && rule.Complexity != "complex" {
			return validationError(rulePath+".complexity", "must be one of: complex, simple")
		}
		if _, exists := seen[rule.Complexity]; exists {
			return validationError(rulePath+".complexity", "duplicates complexity %q", rule.Complexity)
		}
		seen[rule.Complexity] = struct{}{}
		if err := validateTarget(rulePath+".target", rule.Target, providers); err != nil {
			return err
		}
	}
	for _, required := range []string{"simple", "complex"} {
		if _, exists := seen[required]; !exists {
			return validationError(path+".semantic_rules", "missing %q complexity rule", required)
		}
	}
	return nil
}

func validateTarget(path string, target RouteTarget, providers map[string]map[string]struct{}) error {
	if err := validateName(path+".provider", target.Provider); err != nil {
		return err
	}
	models, exists := providers[target.Provider]
	if !exists {
		return validationError(path+".provider", "references unknown provider %q", target.Provider)
	}
	if err := validateName(path+".model", target.Model); err != nil {
		return err
	}
	if _, exists := models[target.Model]; !exists {
		return validationError(path+".model", "references model %q not declared by provider %q", target.Model, target.Provider)
	}
	if target.Weight < 0 {
		return validationError(path+".weight", "must be greater than or equal to 0")
	}
	return nil
}

func validateAuth(cfg AuthConfig, routeModels map[string]struct{}) error {
	names := make(map[string]struct{}, len(cfg.Keys))
	tokens := make(map[string]int, len(cfg.Keys))
	for i, key := range cfg.Keys {
		path := "auth.keys[" + strconv.Itoa(i) + "]"
		if err := validateName(path+".name", key.Name); err != nil {
			return err
		}
		if _, exists := names[key.Name]; exists {
			return validationError(path+".name", "duplicates key name %q", key.Name)
		}
		names[key.Name] = struct{}{}
		if key.Token == "" {
			return validationError(path+".token", "must not be empty")
		}
		if previous, exists := tokens[key.Token]; exists {
			return validationError(path+".token", "duplicates auth.keys[%d].token", previous)
		}
		tokens[key.Token] = i
		if key.Role != "admin" && key.Role != "user" {
			return validationError(path+".role", "must be one of: admin, user")
		}
		if key.DailyTokenLimit < 0 {
			return validationError(path+".daily_token_limit", "must be greater than or equal to 0")
		}
		if key.Models == "" {
			continue
		}
		seenModels := make(map[string]struct{})
		for _, rawModel := range strings.Split(key.Models, ",") {
			model := strings.TrimSpace(rawModel)
			if model == "" {
				return validationError(path+".models", "must be a comma-separated list without empty entries")
			}
			if _, exists := seenModels[model]; exists {
				return validationError(path+".models", "duplicates route model %q", model)
			}
			seenModels[model] = struct{}{}
			if _, exists := routeModels[model]; !exists {
				return validationError(path+".models", "references unknown route model %q", model)
			}
		}
	}
	return nil
}

func validateRateLimit(cfg RateLimitConfig) error {
	if cfg.PerKey < 0 {
		return validationError("rate_limit.per_key", "must be greater than or equal to 0")
	}
	if cfg.PerModel < 0 {
		return validationError("rate_limit.per_model", "must be greater than or equal to 0")
	}
	if cfg.Enabled && cfg.PerKey == 0 && cfg.PerModel == 0 {
		return validationError("rate_limit", "must configure per_key or per_model when enabled")
	}
	return nil
}

func validateCache(cfg CacheConfig) error {
	if cfg.Backend != "memory" && cfg.Backend != "redis" {
		return validationError("cache.backend", "must be one of: memory, redis")
	}
	ttl, err := time.ParseDuration(cfg.TTL)
	if err != nil {
		return validationError("cache.ttl", "must be a valid duration: %v", err)
	}
	if ttl <= 0 {
		return validationError("cache.ttl", "must be greater than 0")
	}
	if cfg.Strategy != "exact" && cfg.Strategy != "semantic" {
		return validationError("cache.strategy", "must be one of: exact, semantic")
	}
	if cfg.MaxSize <= 0 {
		return validationError("cache.max_size", "must be greater than 0")
	}
	if cfg.Threshold <= 0 || cfg.Threshold > 1 {
		return validationError("cache.threshold", "must be greater than 0 and less than or equal to 1")
	}
	if cfg.Backend == "redis" && strings.TrimSpace(cfg.RedisAddr) == "" {
		return validationError("cache.redis_addr", "must not be empty for redis backend")
	}
	if cfg.RedisDB < 0 {
		return validationError("cache.redis_db", "must be greater than or equal to 0")
	}
	return nil
}

func validateTracing(cfg TracingConfig) error {
	if cfg.Exporter != "stdout" && cfg.Exporter != "otlp" {
		return validationError("tracing.exporter", "must be one of: otlp, stdout")
	}
	if strings.TrimSpace(cfg.ServiceName) == "" {
		return validationError("tracing.service_name", "must not be empty")
	}
	if cfg.SampleRatio <= 0 || cfg.SampleRatio > 1 {
		return validationError("tracing.sample_ratio", "must be greater than 0 and less than or equal to 1")
	}
	return nil
}

func validateFilter(cfg FilterConfig) error {
	if cfg.Mode != "mask" && cfg.Mode != "block" {
		return validationError("filter.mode", "must be one of: block, mask")
	}
	seen := make(map[string]struct{}, len(cfg.Rules))
	for i, rule := range cfg.Rules {
		path := "filter.rules[" + strconv.Itoa(i) + "]"
		if _, exists := validFilterRules[rule]; !exists {
			return validationError(path, "unknown filter rule %q", rule)
		}
		if _, exists := seen[rule]; exists {
			return validationError(path, "duplicates filter rule %q", rule)
		}
		seen[rule] = struct{}{}
	}
	if cfg.Enabled && len(cfg.Rules) == 0 {
		return validationError("filter.rules", "must contain at least one rule when filter is enabled")
	}
	return nil
}

func validateBaseURL(path, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return validationError(path, "must be an absolute http or https URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return validationError(path, "must not contain user info, query, or fragment")
	}
	return nil
}

func validateName(path, value string) error {
	if strings.TrimSpace(value) == "" {
		return validationError(path, "must not be empty")
	}
	if value != strings.TrimSpace(value) {
		return validationError(path, "must not have leading or trailing whitespace")
	}
	return nil
}
