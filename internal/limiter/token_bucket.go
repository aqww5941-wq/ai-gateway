package limiter

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type TokenBucketLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rateLimiterEntry
	rate     rate.Limit
	burst    int
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

func NewTokenBucketLimiter(perMinute int) *TokenBucketLimiter {
	burst := perMinute
	if burst < 1 {
		burst = 1
	}
	return &TokenBucketLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		rate:     rate.Limit(float64(perMinute) / 60.0),
		burst:    burst,
	}
}

func (l *TokenBucketLimiter) Allow(key string) bool {
	l.mu.Lock()
	entry, ok := l.limiters[key]
	if !ok {
		entry = &rateLimiterEntry{
			limiter:  rate.NewLimiter(l.rate, l.burst),
			lastUsed: time.Now(),
		}
		l.limiters[key] = entry
	}
	entry.lastUsed = time.Now()
	l.mu.Unlock()

	return entry.limiter.Allow()
}
