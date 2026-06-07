package router

import (
	"context"
	"testing"

	"ai-gateway/internal/provider"
)

// BenchmarkWeightedStrategy measures the per-call cost of the weighted picker.
// Run with: go test -bench=BenchmarkWeightedStrategy -benchmem ./internal/router/
// After the math/rand v2 migration this path is lock-free, so -cpu=8 should
// scale near-linearly.
func BenchmarkWeightedStrategy(b *testing.B) {
	s := &WeightedStrategy{}
	req := &provider.ChatRequest{Model: "x"}
	targets := []Target{
		{Provider: "p1", Model: "m1", Weight: 10},
		{Provider: "p2", Model: "m2", Weight: 30},
		{Provider: "p3", Model: "m3", Weight: 60},
	}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = s.Select(ctx, req, targets)
		}
	})
}

func BenchmarkRoundRobin(b *testing.B) {
	s := &RoundRobinStrategy{}
	req := &provider.ChatRequest{Model: "x"}
	targets := []Target{
		{Provider: "p1", Model: "m1"},
		{Provider: "p2", Model: "m2"},
		{Provider: "p3", Model: "m3"},
	}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = s.Select(ctx, req, targets)
		}
	})
}
