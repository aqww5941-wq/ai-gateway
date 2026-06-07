package middleware

import (
	"crypto/subtle"
	"strconv"
	"testing"
)

// BenchmarkAuthAllow_HashLookup confirms that the SHA-256-indexed lookup is
// O(1) regardless of the configured key count. With the original slice
// implementation, time-per-op grew linearly with len(keys).
func BenchmarkAuthAllow_10Keys(b *testing.B) {
	benchAuth(b, 10)
}

func BenchmarkAuthAllow_1000Keys(b *testing.B) {
	benchAuth(b, 1000)
}

func BenchmarkAuthAllow_10000Keys(b *testing.B) {
	benchAuth(b, 10000)
}

func benchAuth(b *testing.B, n int) {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = "k-" + strconv.Itoa(i)
	}
	keys[n-1] = "winner"
	a := NewAuth(keys)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = a.allow("winner")
	}
}

// --- Baseline (the original O(N) implementation) for direct comparison. ---

func authAllowLinear(token string, keys []string) bool {
	for _, k := range keys {
		if subtle.ConstantTimeCompare([]byte(token), []byte(k)) == 1 {
			return true
		}
	}
	return false
}

func BenchmarkAuthAllow_Linear_10Keys(b *testing.B)    { benchAuthLinear(b, 10) }
func BenchmarkAuthAllow_Linear_1000Keys(b *testing.B)  { benchAuthLinear(b, 1000) }
func BenchmarkAuthAllow_Linear_10000Keys(b *testing.B) { benchAuthLinear(b, 10000) }

func benchAuthLinear(b *testing.B, n int) {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = "k-" + strconv.Itoa(i)
	}
	keys[n-1] = "winner"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = authAllowLinear("winner", keys)
	}
}
