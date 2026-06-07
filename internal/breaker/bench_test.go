package breaker

import (
	"testing"
)

// BenchmarkAllow_ClosedHotPath measures the steady-state cost of the breaker
// when it's closed and traffic is healthy — the path that runs on >99% of
// requests in production. It should be a handful of atomic loads, no allocs.
func BenchmarkAllow_ClosedHotPath(b *testing.B) {
	br := New("bench", Config{FailureThreshold: 100})
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = br.Allow()
			br.OnSuccess()
		}
	})
}

// BenchmarkAllow_OpenShortCircuit measures the cost of the "fast fail" path.
// Should be ~1 atomic load + a time comparison.
func BenchmarkAllow_OpenShortCircuit(b *testing.B) {
	br := New("bench", Config{FailureThreshold: 1, CoolDown: 1 << 30}) // never recover
	br.OnFailure()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = br.Allow()
		}
	})
}
