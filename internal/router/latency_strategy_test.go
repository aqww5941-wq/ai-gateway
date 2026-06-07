package router

import (
	"context"
	"math/rand/v2"
	"testing"
	"time"

	"ai-gateway/internal/provider"
)

// TestLatencyTracker_BasicStats — pumping in known samples should produce
// the expected p99 and failure rate.
func TestLatencyTracker_BasicStats(t *testing.T) {
	tr := NewLatencyTracker(10)
	for i := 0; i < 10; i++ {
		tr.Observe(time.Duration(i+1)*time.Millisecond, i == 9) // last sample fails
	}
	// p99 with 10 samples is index ceil(9.9)-1 = 9 — the largest.
	if got, want := tr.P99(), 10*time.Millisecond; got != want {
		t.Errorf("P99 = %s, want %s", got, want)
	}
	if got := tr.FailureRate(); got != 0.1 {
		t.Errorf("FailureRate = %v, want 0.1", got)
	}
}

// TestLatencyTracker_RingBufferReuse — once the window fills, oldest samples
// should fall off, not accumulate forever.
func TestLatencyTracker_RingBufferReuse(t *testing.T) {
	tr := NewLatencyTracker(3)
	tr.Observe(100*time.Millisecond, false)
	tr.Observe(100*time.Millisecond, false)
	tr.Observe(100*time.Millisecond, false)
	if tr.Samples() != 3 {
		t.Fatalf("Samples = %d, want 3", tr.Samples())
	}
	// Overwrite all 3 with much-faster samples.
	tr.Observe(time.Millisecond, false)
	tr.Observe(time.Millisecond, false)
	tr.Observe(time.Millisecond, false)
	if got := tr.P99(); got != time.Millisecond {
		t.Errorf("P99 after rollover = %s, want 1ms (old samples should be gone)", got)
	}
	if tr.Samples() != 3 {
		t.Fatalf("Samples = %d after rollover, want 3 (capped at window)", tr.Samples())
	}
}

// TestLatencyStrategy_WarmupUsesBaseWeight — until WarmupSamples observations
// are recorded, every target's weight equals its configured base weight.
func TestLatencyStrategy_WarmupUsesBaseWeight(t *testing.T) {
	s := NewLatencyStrategy(64, 5, 0.5)
	targets := []Target{
		{Provider: "fast", Model: "m", Weight: 1},
		{Provider: "slow", Model: "m", Weight: 1},
	}
	// With no observations and equal base weights, picks should be ~50/50.
	picks := map[string]int{}
	for i := 0; i < 1000; i++ {
		got, _ := s.Select(context.Background(), &provider.ChatRequest{}, targets)
		picks[got.Provider]++
	}
	if picks["fast"] < 350 || picks["slow"] < 350 {
		t.Errorf("warmup distribution skewed: %+v (want ~500/500)", picks)
	}
}

// TestLatencyStrategy_PrefersFastProvider — once warmed up, a noticeably
// faster provider should win the lion's share of routings.
func TestLatencyStrategy_PrefersFastProvider(t *testing.T) {
	s := NewLatencyStrategy(64, 5, 0.99)
	fast := Target{Provider: "fast", Model: "m", Weight: 1}
	slow := Target{Provider: "slow", Model: "m", Weight: 1}
	for i := 0; i < 20; i++ {
		s.Observe(fast, 10*time.Millisecond, false)
		s.Observe(slow, 500*time.Millisecond, false)
	}
	picks := map[string]int{}
	for i := 0; i < 2000; i++ {
		got, _ := s.Select(context.Background(), &provider.ChatRequest{}, []Target{fast, slow})
		picks[got.Provider]++
	}
	// Weight ratio is ~50:1 (500ms / 10ms), so we expect "fast" to dominate
	// — at least 5x more than "slow". Generous bound to keep the test flake-free.
	if picks["fast"] < 5*picks["slow"] {
		t.Errorf("fast/slow split = %d/%d, want fast >= 5x slow", picks["fast"], picks["slow"])
	}
}

// TestLatencyStrategy_SkipsFailingProvider — providers above the failure
// fraction get edged out entirely until their failure rate recovers.
func TestLatencyStrategy_SkipsFailingProvider(t *testing.T) {
	s := NewLatencyStrategy(10, 5, 0.5)
	healthy := Target{Provider: "ok", Model: "m", Weight: 1}
	broken := Target{Provider: "broken", Model: "m", Weight: 1}

	// Fill healthy's window with successes, broken's with mostly failures.
	for i := 0; i < 10; i++ {
		s.Observe(healthy, 50*time.Millisecond, false)
		s.Observe(broken, 50*time.Millisecond, i < 8) // 80% failure
	}
	picks := map[string]int{}
	for i := 0; i < 500; i++ {
		got, _ := s.Select(context.Background(), &provider.ChatRequest{}, []Target{healthy, broken})
		picks[got.Provider]++
	}
	if picks["broken"] != 0 {
		t.Errorf("broken provider chosen %d times, want 0 (failure rate > threshold)", picks["broken"])
	}
}

// TestLatencyStrategy_FallsBackWhenAllFailing — if every target is unhealthy,
// the strategy should still return ONE rather than refusing to route.
// The breaker layer will fast-fail the actual call.
func TestLatencyStrategy_FallsBackWhenAllFailing(t *testing.T) {
	s := NewLatencyStrategy(10, 5, 0.5)
	a := Target{Provider: "a", Model: "m", Weight: 1}
	b := Target{Provider: "b", Model: "m", Weight: 1}
	for i := 0; i < 10; i++ {
		s.Observe(a, 50*time.Millisecond, true)
		s.Observe(b, 50*time.Millisecond, true)
	}
	for i := 0; i < 10; i++ {
		got, err := s.Select(context.Background(), &provider.ChatRequest{}, []Target{a, b})
		if err != nil {
			t.Fatalf("Select errored when all targets unhealthy: %v", err)
		}
		if got == nil {
			t.Fatal("Select returned nil target")
		}
	}
}

// TestRouter_LatencyStrategyFor — only routes that actually use latency
// return a non-nil strategy.
func TestRouter_LatencyStrategyFor(t *testing.T) {
	_ = rand.IntN // keep deterministic test ordering — no side effects
	t.Run("returns strategy for latency route", func(t *testing.T) {
		r := &Router{routes: map[string]*routeEntry{
			"m": {strategy: NewLatencyStrategy(0, 0, 0)},
		}}
		if r.LatencyStrategyFor("m") == nil {
			t.Fatal("expected non-nil for latency route")
		}
	})
	t.Run("returns nil for round_robin", func(t *testing.T) {
		r := &Router{routes: map[string]*routeEntry{
			"m": {strategy: &RoundRobinStrategy{}},
		}}
		if r.LatencyStrategyFor("m") != nil {
			t.Fatal("expected nil for round_robin")
		}
	})
	t.Run("returns nil for unknown model", func(t *testing.T) {
		r := &Router{routes: map[string]*routeEntry{}}
		if r.LatencyStrategyFor("missing") != nil {
			t.Fatal("expected nil for missing route")
		}
	})
}
