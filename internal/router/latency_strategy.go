// Package router — latency strategy.
//
// LatencyStrategy picks a target with probability inversely proportional to
// its recent p99 response time, so faster providers naturally absorb more
// traffic and lagging ones bleed off. It also accounts for failures: a
// provider whose recent failure rate is above FailFraction is skipped
// entirely until its window of failures rolls off.
//
// Comparison with WeightedStrategy:
//
//	WeightedStrategy uses static config-defined weights — operator must
//	tune them by hand and they don't track reality.
//
//	LatencyStrategy adapts to observed latency every request, with no
//	configuration knobs other than the window size. It DOES still respect
//	a configured base Weight, by multiplying the dynamic factor into it,
//	so operators can still bias traffic.
package router

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"ai-gateway/internal/provider"
)

// LatencyTracker is the per-target sliding-window stat holder. It records the
// last WindowSize samples and the last WindowSize failure flags; lookups
// (P99, FailureRate) are O(WindowSize) but that's fine — WindowSize is small
// (default 64) and lookups happen at most once per routing decision.
//
// All ops are guarded by a single Mutex. We could shard, but the contention
// is per-target (one mutex per provider), not global, so this is fine.
type LatencyTracker struct {
	mu         sync.Mutex
	windowSize int
	latencies  []time.Duration // ring buffer
	failures   []bool          // ring buffer, parallel
	idx        int
	count      int
}

// NewLatencyTracker builds a tracker with the given ring-buffer capacity.
// 64 is a reasonable default — large enough that p99 is meaningful, small
// enough that the buffer stays cheap.
func NewLatencyTracker(windowSize int) *LatencyTracker {
	if windowSize <= 0 {
		windowSize = 64
	}
	return &LatencyTracker{
		windowSize: windowSize,
		latencies:  make([]time.Duration, windowSize),
		failures:   make([]bool, windowSize),
	}
}

// Observe records one outcome. Call this from the request path after every
// upstream call.
func (t *LatencyTracker) Observe(d time.Duration, failed bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.latencies[t.idx] = d
	t.failures[t.idx] = failed
	t.idx = (t.idx + 1) % t.windowSize
	if t.count < t.windowSize {
		t.count++
	}
}

// P99 returns an approximate 99th-percentile latency. With 64 samples this
// is really "second-worst" — close enough to p99 for routing purposes, and
// far cheaper than a real online quantile algorithm.
//
// Returns zero if the window is empty.
func (t *LatencyTracker) P99() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.count == 0 {
		return 0
	}
	snap := make([]time.Duration, t.count)
	copy(snap, t.latencies[:t.count])
	// Partial-sort: we only need the top ~1%, so a full sort would be wasteful
	// for large windows. For small windows this is fast enough that the
	// optimization doesn't pay back the complexity.
	// We sort with a simple insertion order pivot — Go's sort.Slice is fine.
	for i := 1; i < len(snap); i++ {
		j := i
		for j > 0 && snap[j-1] > snap[j] {
			snap[j-1], snap[j] = snap[j], snap[j-1]
			j--
		}
	}
	// 99th percentile index, clamped to last element.
	idx := int(math.Ceil(0.99*float64(len(snap)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(snap) {
		idx = len(snap) - 1
	}
	return snap[idx]
}

// FailureRate returns failures / samples in the current window. Returns 0
// when no samples have been recorded yet — a fresh provider gets the benefit
// of the doubt.
func (t *LatencyTracker) FailureRate() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.count == 0 {
		return 0
	}
	fails := 0
	for i := 0; i < t.count; i++ {
		if t.failures[i] {
			fails++
		}
	}
	return float64(fails) / float64(t.count)
}

// Samples returns how many observations have been recorded (caps at WindowSize).
func (t *LatencyTracker) Samples() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.count
}

// LatencyStrategy selects a target with weight ∝ baseWeight / p99_latency.
//
// Until a provider has accumulated WarmupSamples observations, it uses a
// neutral weight equal to baseWeight — otherwise a fresh provider's empty
// window (p99=0) would either divide-by-zero or cheat with infinite weight.
//
// Providers whose recent failure rate is above FailFraction are temporarily
// skipped. If ALL providers are above the threshold (everything is on fire),
// we fall through to picking the least-bad one rather than failing routing.
type LatencyStrategy struct {
	mu             sync.RWMutex
	trackers       map[string]*LatencyTracker // key = "provider/model"
	windowSize     int
	warmupSamples  int
	failFraction   float64
}

// NewLatencyStrategy builds the strategy. Sensible defaults are applied for
// any zero values: windowSize=64, warmupSamples=5, failFraction=0.5.
func NewLatencyStrategy(windowSize, warmupSamples int, failFraction float64) *LatencyStrategy {
	if windowSize <= 0 {
		windowSize = 64
	}
	if warmupSamples <= 0 {
		warmupSamples = 5
	}
	if failFraction <= 0 || failFraction > 1 {
		failFraction = 0.5
	}
	return &LatencyStrategy{
		trackers:      make(map[string]*LatencyTracker),
		windowSize:    windowSize,
		warmupSamples: warmupSamples,
		failFraction:  failFraction,
	}
}

func trackerKey(t Target) string { return t.Provider + "/" + t.Model }

// tracker fetches (or lazily creates) the per-target tracker. Common path
// uses RLock; only the first observation for a given target acquires the
// write lock.
func (s *LatencyStrategy) tracker(key string) *LatencyTracker {
	s.mu.RLock()
	t, ok := s.trackers[key]
	s.mu.RUnlock()
	if ok {
		return t
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok = s.trackers[key]; ok {
		return t
	}
	t = NewLatencyTracker(s.windowSize)
	s.trackers[key] = t
	return t
}

// Observe is called by the server after each upstream call so the strategy
// can adapt. Safe to call on any goroutine.
func (s *LatencyStrategy) Observe(t Target, d time.Duration, failed bool) {
	s.tracker(trackerKey(t)).Observe(d, failed)
}

// Select chooses one target by weighted random where weight is
// baseWeight / max(p99, minLatencyFloor). The minLatencyFloor (1 ms) keeps
// the math sane and prevents a single ultra-fast outlier from monopolizing
// traffic.
func (s *LatencyStrategy) Select(ctx context.Context, req *provider.ChatRequest, targets []Target) (*Target, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets")
	}
	const minLatencyFloor = time.Millisecond

	type candidate struct {
		idx    int
		weight float64
	}
	healthy := make([]candidate, 0, len(targets))
	all := make([]candidate, 0, len(targets))

	for i, t := range targets {
		tr := s.tracker(trackerKey(t))
		base := float64(t.Weight)
		if base <= 0 {
			base = 1
		}
		// Warmup: until we have N samples, use the static base weight.
		if tr.Samples() < s.warmupSamples {
			all = append(all, candidate{idx: i, weight: base})
			healthy = append(healthy, candidate{idx: i, weight: base})
			continue
		}
		p99 := tr.P99()
		if p99 < minLatencyFloor {
			p99 = minLatencyFloor
		}
		w := base / p99.Seconds()
		all = append(all, candidate{idx: i, weight: w})
		if tr.FailureRate() < s.failFraction {
			healthy = append(healthy, candidate{idx: i, weight: w})
		}
	}

	pool := healthy
	if len(pool) == 0 {
		// Everyone is failing — pick from all anyway, weighted by current latency.
		// Routing should never refuse to serve when the failure is upstream-wide;
		// the breaker layer will short-circuit individual calls.
		pool = all
	}
	total := 0.0
	for _, c := range pool {
		total += c.weight
	}
	if total <= 0 {
		return &targets[pool[0].idx], nil
	}
	threshold := rand.Float64() * total
	cum := 0.0
	for _, c := range pool {
		cum += c.weight
		if cum >= threshold {
			return &targets[c.idx], nil
		}
	}
	return &targets[pool[len(pool)-1].idx], nil
}

// LatencyObserver is the narrow interface the server uses to feed latency
// samples back without coupling to the concrete strategy. The router exposes
// a Get method that returns this if the matched route uses LatencyStrategy.
type LatencyObserver interface {
	Observe(t Target, d time.Duration, failed bool)
}

// LatencyStrategyFor returns the LatencyStrategy attached to the route for
// reqModel, or nil if the route doesn't use it. Used by the server to feed
// observations back into the strategy.
func (r *Router) LatencyStrategyFor(reqModel string) *LatencyStrategy {
	r.mu.RLock()
	entry, ok := r.routes[reqModel]
	r.mu.RUnlock()
	if !ok {
		return nil
	}
	if ls, ok := entry.strategy.(*LatencyStrategy); ok {
		return ls
	}
	return nil
}
