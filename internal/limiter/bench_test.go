package limiter

import (
	"strconv"
	"sync/atomic"
	"testing"
)

// BenchmarkAllow_HotKey measures the per-call cost when a single key is
// shared by all goroutines. After the sync.Map migration this path is
// lock-free in the common case (key already present), so contention here
// stays low.
func BenchmarkAllow_HotKey(b *testing.B) {
	l := NewTokenBucketLimiter(1_000_000_000) // effectively unlimited
	defer l.Close()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Allow("hot")
		}
	})
}

// BenchmarkAllow_ManyKeys mimics a multi-tenant gateway: each goroutine
// rotates through ~1k distinct keys, exercising sync.Map's read path.
func BenchmarkAllow_ManyKeys(b *testing.B) {
	l := NewTokenBucketLimiter(1_000_000_000)
	defer l.Close()

	// Pre-warm so we measure the steady-state lookup, not creation.
	for i := 0; i < 1024; i++ {
		l.Allow("k-" + strconv.Itoa(i))
	}

	var ctr atomic.Uint64
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			n := ctr.Add(1) & 1023
			l.Allow("k-" + strconv.Itoa(int(n)))
		}
	})
}
