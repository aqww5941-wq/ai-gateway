package router

import (
	"context"
	"fmt"

	"ai-gateway/internal/provider"
)

// FallbackStrategy selects targets in priority order for failover.
type FallbackStrategy struct{}

func (s *FallbackStrategy) Select(ctx context.Context, req *provider.ChatRequest, targets []Target) (*Target, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets")
	}
	// Return the first (highest priority) target.
	// The caller retries with the next target on failure via GetFallbackChain.
	return &targets[0], nil
}

// GetFallbackChain returns targets in priority order for failover.
func (s *FallbackStrategy) GetFallbackChain(targets []Target) []Target {
	return targets
}
