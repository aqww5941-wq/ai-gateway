package router

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"

	"ai-gateway/config"
	"ai-gateway/internal/provider"
)

type Target struct {
	Provider string
	Model    string
	Weight   int
}

type RoutingStrategy interface {
	Select(ctx context.Context, req *provider.ChatRequest, targets []Target) (*Target, error)
}

type WeightedStrategy struct {
	rng *rand.Rand
	mu  sync.Mutex
}

func (s *WeightedStrategy) Select(ctx context.Context, req *provider.ChatRequest, targets []Target) (*Target, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets")
	}
	s.mu.Lock()
	r := s.rng.Float64()
	s.mu.Unlock()

	totalWeight := 0
	for _, t := range targets {
		totalWeight += t.Weight
	}

	if totalWeight == 0 {
		return &targets[0], nil
	}

	cumulative := 0.0
	threshold := r * float64(totalWeight)
	for i, t := range targets {
		cumulative += float64(t.Weight)
		if cumulative >= threshold {
			return &targets[i], nil
		}
	}

	return &targets[len(targets)-1], nil
}

type RoundRobinStrategy struct {
	counter atomic.Uint64
}

func (s *RoundRobinStrategy) Select(ctx context.Context, req *provider.ChatRequest, targets []Target) (*Target, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets")
	}
	idx := s.counter.Add(1) - 1
	return &targets[idx%uint64(len(targets))], nil
}

type routeEntry struct {
	strategy RoutingStrategy
	targets  []Target
}

type Router struct {
	mu     sync.RWMutex
	routes map[string]*routeEntry
}

func NewRouter(configs []config.RouteConfig) (*Router, error) {
	r := &Router{routes: make(map[string]*routeEntry)}
	for _, cfg := range configs {
		if len(cfg.Targets) == 0 {
			return nil, fmt.Errorf("route %q: no targets", cfg.Name)
		}
		targets := make([]Target, len(cfg.Targets))
		for i, t := range cfg.Targets {
			targets[i] = Target{Provider: t.Provider, Model: t.Model}
			if cfg.Strategy == "weighted" {
				targets[i].Weight = t.Weight
			}
		}
		var strategy RoutingStrategy
		switch cfg.Strategy {
		case "round_robin":
			strategy = &RoundRobinStrategy{}
		case "weighted":
			strategy = &WeightedStrategy{rng: rand.New(rand.NewSource(rand.Int63()))}
		case "fallback":
			strategy = &FallbackStrategy{}
		case "semantic":
			s, err := NewSemanticStrategy(cfg.SemanticRules)
			if err != nil {
				return nil, fmt.Errorf("route %q: %w", cfg.Name, err)
			}
			strategy = s
		default:
			return nil, fmt.Errorf("route %q: unknown strategy %q", cfg.Name, cfg.Strategy)
		}
		r.routes[cfg.Match.Model] = &routeEntry{strategy: strategy, targets: targets}
	}
	return r, nil
}

func (r *Router) Route(ctx context.Context, req *provider.ChatRequest) (*Target, error) {
	r.mu.RLock()
	entry, ok := r.routes[req.Model]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no route for model %q", req.Model)
	}
	return entry.strategy.Select(ctx, req, entry.targets)
}

// FallbackChain returns all targets in priority order for fallback routing.
// Returns nil if the route doesn't use a fallback strategy.
func (r *Router) FallbackChain(reqModel string) []Target {
	r.mu.RLock()
	entry, ok := r.routes[reqModel]
	r.mu.RUnlock()
	if !ok {
		return nil
	}
	if fs, ok := entry.strategy.(*FallbackStrategy); ok {
		return fs.GetFallbackChain(entry.targets)
	}
	return nil
}
