package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"

	"ai-gateway/config"
	"ai-gateway/internal/breaker"
	"ai-gateway/internal/cache"
	"ai-gateway/internal/metrics"
	"ai-gateway/internal/provider"
	"ai-gateway/internal/router"
)

type runtimeSnapshot struct {
	revision  uint64
	config    *config.Config
	router    *router.Router
	providers map[string]provider.LLMProvider
	breakers  *breaker.Manager
}

type runtimeState struct {
	current  atomic.Pointer[runtimeSnapshot]
	reloadMu sync.Mutex
}

func newInitialRuntimeSnapshot(cfg *config.Config, r *router.Router, providers map[string]provider.LLMProvider) *runtimeSnapshot {
	return &runtimeSnapshot{
		revision:  1,
		config:    config.Clone(cfg),
		router:    r,
		providers: cloneProviderRegistry(providers),
		breakers:  breaker.NewManager(breaker.Config{}),
	}
}

func cloneProviderRegistry(source map[string]provider.LLMProvider) map[string]provider.LLMProvider {
	clone := make(map[string]provider.LLMProvider, len(source))
	for name, instance := range source {
		clone[name] = instance
	}
	return clone
}

func (s *Server) currentSnapshot() *runtimeSnapshot {
	return s.runtime.current.Load()
}

func (snapshot *runtimeSnapshot) cacheKey(req *provider.ChatRequest) string {
	return fmt.Sprintf("revision:%d:%s", snapshot.revision, cache.CacheKey(req))
}

func buildRuntimeSnapshot(cfg *config.Config, revision uint64, changedSections []string, current *runtimeSnapshot, transport *http.Transport, logger *slog.Logger) (*runtimeSnapshot, error) {
	providers := current.providers
	breakers := current.breakers
	providersChanged := containsSection(changedSections, "providers")
	if providersChanged {
		providers = make(map[string]provider.LLMProvider, len(cfg.Providers))
		for _, providerConfig := range cfg.Providers {
			if !providerConfig.RuntimeEnabled() {
				continue
			}
			providerLogger := logger.With("provider", providerConfig.Name)
			var instance provider.LLMProvider
			var err error
			switch providerConfig.Type {
			case "openai":
				instance, err = provider.NewOpenAI(providerConfig, providerLogger)
			case "claude":
				instance, err = provider.NewClaude(providerConfig, providerLogger)
			default:
				return nil, fmt.Errorf("provider %q: unknown provider type %q", providerConfig.Name, providerConfig.Type)
			}
			if err != nil {
				return nil, fmt.Errorf("provider %q: %w", providerConfig.Name, err)
			}
			if setter, ok := instance.(provider.TransportSetter); ok {
				setter.SetTransport(transport)
			}
			providers[providerConfig.Name] = instance
		}
		if len(providers) == 0 {
			return nil, fmt.Errorf("no runnable providers configured: native bootstrap providers must remain disabled until their adapters are implemented")
		}
		breakers = breaker.NewManager(breaker.Config{})
	}

	runtimeRouter := current.router
	if providersChanged || containsSection(changedSections, "routes") {
		var err error
		runtimeRouter, err = router.NewRouter(cfg.Routes)
		if err != nil {
			return nil, fmt.Errorf("router: %w", err)
		}
	}
	return &runtimeSnapshot{
		revision:  revision,
		config:    cfg,
		router:    runtimeRouter,
		providers: providers,
		breakers:  breakers,
	}, nil
}

func containsSection(sections []string, expected string) bool {
	for _, section := range sections {
		if section == expected {
			return true
		}
	}
	return false
}

// Reload validates and builds a complete candidate before one atomic publish.
// Only providers/routes are dynamic in M0; all startup-built sections require
// a restart and leave the active snapshot untouched.
func (s *Server) Reload(candidate *config.Config) error {
	s.runtime.reloadMu.Lock()
	defer s.runtime.reloadMu.Unlock()

	current := s.currentSnapshot()
	if current == nil {
		return fmt.Errorf("runtime snapshot is not initialized")
	}
	immutableCandidate := config.Clone(candidate)
	nextRevision := current.revision + 1
	dynamicSections := config.DynamicReloadSections(current.config, immutableCandidate)
	s.logger.Info("runtime config reload started",
		"from_revision", current.revision,
		"candidate_revision", nextRevision,
		"dynamic_sections", dynamicSections,
	)

	if err := config.Validate(immutableCandidate); err != nil {
		metrics.ConfigReloadTotal.WithLabelValues("rejected", "validation").Inc()
		s.logger.Warn("runtime config reload rejected",
			"from_revision", current.revision,
			"candidate_revision", nextRevision,
			"stage", "validation",
			"error", err,
		)
		return fmt.Errorf("validate reload candidate: %w", err)
	}
	if err := config.ValidateReloadBoundary(current.config, immutableCandidate); err != nil {
		metrics.ConfigReloadTotal.WithLabelValues("rejected", "restart_required").Inc()
		sections, _ := config.RestartRequiredSections(err)
		s.logger.Warn("runtime config reload rejected",
			"from_revision", current.revision,
			"candidate_revision", nextRevision,
			"stage", "restart_required",
			"changed_sections", sections,
			"error", err,
		)
		return err
	}
	dynamicSections = config.DynamicReloadSections(current.config, immutableCandidate)
	if len(dynamicSections) == 0 {
		metrics.ConfigReloadTotal.WithLabelValues("unchanged", "compare").Inc()
		s.logger.Info("runtime config reload unchanged", "config_revision", current.revision)
		return nil
	}

	next, err := buildRuntimeSnapshot(immutableCandidate, nextRevision, dynamicSections, current, s.transport, s.logger)
	if err != nil {
		metrics.ConfigReloadTotal.WithLabelValues("rejected", "snapshot_build").Inc()
		s.logger.Warn("runtime config reload rejected",
			"from_revision", current.revision,
			"candidate_revision", nextRevision,
			"stage", "snapshot_build",
			"error", err,
		)
		return fmt.Errorf("build runtime snapshot: %w", err)
	}

	s.runtime.current.Store(next)
	metrics.ConfigReloadTotal.WithLabelValues("published", "publish").Inc()
	s.logger.Info("runtime config reload published",
		"from_revision", current.revision,
		"config_revision", next.revision,
		"changed_sections", dynamicSections,
	)
	return nil
}
