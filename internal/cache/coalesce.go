// Package coalesce wraps a CacheBackend with singleflight so concurrent
// identical requests share a single upstream call.
//
// Why this matters for an LLM gateway:
//
// Without coalescing, a "popular" prompt that briefly leaves the cache (e.g.
// just after a TTL expiry, or during a cold start) can trigger N simultaneous
// upstream calls — N being however many concurrent requests happen to land
// in the gap. For LLM providers this means paying N times for the same
// inference and burning N units of upstream rate-limit budget. With
// singleflight, only the first caller actually hits the upstream; the others
// block until that single in-flight result is returned to all of them.
//
// We coalesce on the cache key (already a deterministic hash of model +
// messages + sampling params), so dedup is exact, not heuristic.
package cache

import (
	"context"
	"sync/atomic"

	"ai-gateway/internal/provider"

	"golang.org/x/sync/singleflight"
)

// Coalescer dedups concurrent cache-miss work behind a shared upstream call.
// The zero value is usable but exposes no metrics; use NewCoalescer for full
// hit/share counters.
type Coalescer struct {
	g      singleflight.Group
	calls  atomic.Uint64 // total Do invocations
	shared atomic.Uint64 // requests that piggybacked on another in-flight call
}

func NewCoalescer() *Coalescer { return &Coalescer{} }

// Do executes fn under the singleflight key, sharing the result among all
// concurrent callers with the same key. The bool return indicates whether
// this caller's result was shared from another in-flight call (true) versus
// being the call leader (false).
func (c *Coalescer) Do(ctx context.Context, key string, fn func(ctx context.Context) (*provider.ChatResponse, error)) (*provider.ChatResponse, bool, error) {
	c.calls.Add(1)
	v, err, shared := c.g.Do(key, func() (any, error) {
		return fn(ctx)
	})
	if shared {
		c.shared.Add(1)
	}
	if err != nil {
		return nil, shared, err
	}
	if v == nil {
		return nil, shared, nil
	}
	return v.(*provider.ChatResponse), shared, nil
}

// Stats returns (total_calls, shared_calls). shared_calls / total_calls is
// the "dedup ratio" — under load this is the bandwidth/cost saved.
func (c *Coalescer) Stats() (uint64, uint64) {
	return c.calls.Load(), c.shared.Load()
}
