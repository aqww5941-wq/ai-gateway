package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// RestartRequiredError reports bootstrap sections that cannot be changed by
// the current M0 runtime. Callers can classify it with errors.As instead of
// matching the human-readable message.
type RestartRequiredError struct {
	Sections []string
}

func (e *RestartRequiredError) Error() string {
	return "config reload requires restart for sections: " + strings.Join(e.Sections, ", ")
}

// RestartRequiredSections returns a defensive copy of the rejected sections.
func RestartRequiredSections(err error) ([]string, bool) {
	var restartErr *RestartRequiredError
	if !errors.As(err, &restartErr) {
		return nil, false
	}
	return append([]string(nil), restartErr.Sections...), true
}

// ValidateReloadBoundary enforces the M0 dynamic/restart-required contract.
// Providers and routes are dynamic; every other top-level section owns
// startup-built resources and therefore requires a process restart.
func ValidateReloadBoundary(current, candidate *Config) error {
	if current == nil || candidate == nil {
		return fmt.Errorf("reload boundary requires non-nil current and candidate configs")
	}

	sections := make([]string, 0, 7)
	if !reflect.DeepEqual(current.Server, candidate.Server) {
		sections = append(sections, "server")
	}
	if !reflect.DeepEqual(current.Auth, candidate.Auth) {
		sections = append(sections, "auth")
	}
	if !reflect.DeepEqual(current.RateLimit, candidate.RateLimit) {
		sections = append(sections, "rate_limit")
	}
	if !reflect.DeepEqual(current.Quota, candidate.Quota) {
		sections = append(sections, "quota")
	}
	if !reflect.DeepEqual(current.Cache, candidate.Cache) {
		sections = append(sections, "cache")
	}
	if !reflect.DeepEqual(current.Tracing, candidate.Tracing) {
		sections = append(sections, "tracing")
	}
	if !reflect.DeepEqual(current.Filter, candidate.Filter) {
		sections = append(sections, "filter")
	}
	if len(sections) != 0 {
		return &RestartRequiredError{Sections: sections}
	}
	return nil
}

// DynamicReloadSections reports the current M0 dynamic sections that differ.
func DynamicReloadSections(current, candidate *Config) []string {
	if current == nil || candidate == nil {
		return nil
	}
	sections := make([]string, 0, 2)
	if !reflect.DeepEqual(current.Providers, candidate.Providers) {
		sections = append(sections, "providers")
	}
	if !reflect.DeepEqual(current.Routes, candidate.Routes) {
		sections = append(sections, "routes")
	}
	return sections
}

// Clone returns a deep copy suitable for immutable runtime publication.
func Clone(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	clone.Auth.Keys = append([]KeyConfig(nil), cfg.Auth.Keys...)
	clone.Filter.Rules = append([]string(nil), cfg.Filter.Rules...)

	clone.Providers = make([]ProviderConfig, len(cfg.Providers))
	for i, source := range cfg.Providers {
		clone.Providers[i] = source
		clone.Providers[i].Models = append([]string(nil), source.Models...)
		if source.Enabled != nil {
			enabled := *source.Enabled
			clone.Providers[i].Enabled = &enabled
		}
		if source.Ark != nil {
			ark := *source.Ark
			clone.Providers[i].Ark = &ark
		}
		if source.DeepSeek != nil {
			deepseek := *source.DeepSeek
			clone.Providers[i].DeepSeek = &deepseek
		}
		if source.Qwen != nil {
			qwen := *source.Qwen
			clone.Providers[i].Qwen = &qwen
		}
	}

	clone.Routes = make([]RouteConfig, len(cfg.Routes))
	for i, source := range cfg.Routes {
		clone.Routes[i] = source
		clone.Routes[i].Targets = append([]RouteTarget(nil), source.Targets...)
		clone.Routes[i].SemanticRules = append([]SemanticRuleConfig(nil), source.SemanticRules...)
	}
	return &clone
}
