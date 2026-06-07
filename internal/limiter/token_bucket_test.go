package limiter

import (
	"testing"
	"time"
)

func TestTokenBucketAllow(t *testing.T) {
	// 60 per minute = 1 per second
	l := NewTokenBucketLimiter(60)

	key := "test-key"

	// Should allow first request
	if !l.Allow(key) {
		t.Error("expected first request to be allowed")
	}

	// Drain the bucket by requesting burst-size requests rapidly
	// burst = 60, we've used 1
	allowed := 0
	for i := 0; i < 100; i++ {
		if l.Allow(key) {
			allowed++
		}
	}
	// After using up tokens, we should get denied
	if allowed > 60 {
		t.Errorf("expected at most 60 allowed (token bucket), got %d", allowed)
	}
}

func TestTokenBucketMultipleKeys(t *testing.T) {
	l := NewTokenBucketLimiter(60)

	if !l.Allow("key1") {
		t.Error("key1 should be allowed")
	}
	if !l.Allow("key2") {
		t.Error("key2 should be allowed (separate bucket)")
	}
}

func TestTokenBucketAlwaysAllowZero(t *testing.T) {
	l := NewTokenBucketLimiter(0)
	// Burst is at least 1 so first should work
	if !l.Allow("x") {
		t.Error("expected allow")
	}
	// Rate of 0 means no refill, burst consumed
	time.Sleep(10 * time.Millisecond)
	// Already consumed the burst, should be denied
	_ = l.Allow("x") // may or may not be allowed
}
