package limiter

import (
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// TokenBucketLimiter holds a per-key token bucket. Limiters are created lazily
// and a background sweeper evicts entries that have not been used recently to
// bound memory under high-cardinality key spaces (per-IP, per-API-key, etc).
//
// The hot Allow path uses sync.Map.Load → atomic timestamp update, which is
// lock-free for the common case (key already present). Limiter creation falls
// back to LoadOrStore so two goroutines hitting a new key share a bucket.
type TokenBucketLimiter struct {
	limiters sync.Map // key string -> *rateLimiterEntry
	rate     rate.Limit
	burst    int

	stopCh   chan struct{}
	stopOnce sync.Once
}

type rateLimiterEntry struct {
	limiter      *rate.Limiter
	lastUsedUnix atomic.Int64
}

const (
	defaultIdleTimeout = 10 * time.Minute
	defaultSweepEvery  = 1 * time.Minute
)

func NewTokenBucketLimiter(perMinute int) *TokenBucketLimiter {
	burst := perMinute
	if burst < 1 {
		burst = 1
	}
	l := &TokenBucketLimiter{
		rate:   rate.Limit(float64(perMinute) / 60.0),
		burst:  burst,
		stopCh: make(chan struct{}),
	}
	go l.sweepLoop(defaultSweepEvery, defaultIdleTimeout)
	return l
}

// Close stops the background sweeper. Safe to call multiple times.
func (l *TokenBucketLimiter) Close() {
	l.stopOnce.Do(func() { close(l.stopCh) })
}

func (l *TokenBucketLimiter) Allow(key string) bool {
	now := time.Now().Unix()

	if v, ok := l.limiters.Load(key); ok {
		entry := v.(*rateLimiterEntry)
		entry.lastUsedUnix.Store(now)
		return entry.limiter.Allow()
	}

	// Slow path: create a new entry, but use LoadOrStore so concurrent
	// first-touches collapse onto a single limiter.
	fresh := &rateLimiterEntry{limiter: rate.NewLimiter(l.rate, l.burst)}
	fresh.lastUsedUnix.Store(now)
	actual, _ := l.limiters.LoadOrStore(key, fresh)
	entry := actual.(*rateLimiterEntry)
	entry.lastUsedUnix.Store(now)
	return entry.limiter.Allow()
}

func (l *TokenBucketLimiter) sweepLoop(every, idle time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-idle).Unix()
			l.limiters.Range(func(k, v any) bool {
				entry := v.(*rateLimiterEntry)
				if entry.lastUsedUnix.Load() < cutoff {
					l.limiters.Delete(k)
				}
				return true
			})
		}
	}
}
