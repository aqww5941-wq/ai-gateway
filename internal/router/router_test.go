package router

import (
	"context"
	"math/rand"
	"testing"

	"ai-gateway/internal/provider"
)

func TestWeightedStrategy(t *testing.T) {
	s := &WeightedStrategy{rng: rand.New(rand.NewSource(42))}

	req := &provider.ChatRequest{Model: "gpt-4o"}
	targets := []Target{
		{Provider: "p1", Model: "m1", Weight: 50},
		{Provider: "p2", Model: "m2", Weight: 50},
	}

	counts := map[string]int{}
	for i := 0; i < 1000; i++ {
		target, err := s.Select(context.Background(), req, targets)
		if err != nil {
			t.Fatal(err)
		}
		counts[target.Provider]++
	}

	if counts["p1"] == 0 || counts["p2"] == 0 {
		t.Errorf("both providers should be selected: %v", counts)
	}
}

func TestRoundRobinStrategy(t *testing.T) {
	s := &RoundRobinStrategy{}
	req := &provider.ChatRequest{Model: "gpt-4o"}
	targets := []Target{
		{Provider: "p1", Model: "m1"},
		{Provider: "p2", Model: "m2"},
		{Provider: "p3", Model: "m3"},
	}

	for i := 0; i < 9; i++ {
		target, err := s.Select(context.Background(), req, targets)
		if err != nil {
			t.Fatal(err)
		}
		expected := targets[i%3].Provider
		if target.Provider != expected {
			t.Errorf("round %d: expected %q, got %q", i, expected, target.Provider)
		}
	}
}

func TestFallbackStrategy(t *testing.T) {
	s := &FallbackStrategy{}
	req := &provider.ChatRequest{Model: "gpt-4o"}
	targets := []Target{
		{Provider: "p1", Model: "m1"},
		{Provider: "p2", Model: "m2"},
	}

	target, err := s.Select(context.Background(), req, targets)
	if err != nil {
		t.Fatal(err)
	}
	if target.Provider != "p1" {
		t.Errorf("expected first target p1, got %s", target.Provider)
	}

	chain := s.GetFallbackChain(targets)
	if len(chain) != 2 || chain[0].Provider != "p1" || chain[1].Provider != "p2" {
		t.Errorf("fallback chain incorrect: %v", chain)
	}
}
