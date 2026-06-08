package middleware

import (
	"crypto/subtle"
	"strconv"
	"testing"

	"ai-gateway/internal/store"
)

func BenchmarkAuthLookup_10Keys(b *testing.B)  { benchAuthLookup(b, 10) }
func BenchmarkAuthLookup_100Keys(b *testing.B) { benchAuthLookup(b, 100) }

func benchAuthLookup(b *testing.B, n int) {
	st, err := store.Open(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer st.Close()

	token := ""
	for i := range n {
		t, err := st.CreateKey("k-"+strconv.Itoa(i), "user", "", 0, 0)
		if err != nil {
			b.Fatal(err)
		}
		if i == n-1 {
			token = t
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = st.LookupIdentity(token)
	}
}

// --- Baseline linear comparison ---

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
