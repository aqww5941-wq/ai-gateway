package router

import (
	"context"
	"testing"
	"time"

	"ai-gateway/internal/provider"
)

// BenchmarkLatencyStrategy_Select measures the cost of one routing decision
// after the trackers have been warmed up. The previous WeightedStrategy was
// ~5 ns/op; this strategy is necessarily heavier because it computes p99
// from a sliding window, but should still stay sub-microsecond.
func BenchmarkLatencyStrategy_Select(b *testing.B) {
	s := NewLatencyStrategy(64, 5, 0.5)
	targets := []Target{
		{Provider: "a", Model: "m", Weight: 1},
		{Provider: "b", Model: "m", Weight: 1},
		{Provider: "c", Model: "m", Weight: 1},
	}
	for i := 0; i < 32; i++ {
		s.Observe(targets[0], 100*time.Millisecond, false)
		s.Observe(targets[1], 200*time.Millisecond, false)
		s.Observe(targets[2], 50*time.Millisecond, false)
	}
	req := &provider.ChatRequest{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.Select(context.Background(), req, targets)
	}
}
