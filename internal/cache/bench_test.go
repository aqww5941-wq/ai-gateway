package cache

import (
	"strconv"
	"testing"
	"time"

	"ai-gateway/internal/provider"
)

// BenchmarkLRUSet measures Set under no-evict and steady-state evict modes.
// Compare against a hypothetical naive implementation: the LRU operations
// here are O(1) regardless of cache size.
func BenchmarkLRUSet_NoEvict(b *testing.B) {
	c := NewMemoryCache(b.N+1, time.Hour)
	defer c.Close()
	resp := &provider.ChatResponse{Usage: provider.Usage{TotalTokens: 1}}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(strconv.Itoa(i), resp)
	}
}

func BenchmarkLRUSet_SteadyEvict(b *testing.B) {
	const size = 1000
	c := NewMemoryCache(size, time.Hour)
	defer c.Close()
	resp := &provider.ChatResponse{Usage: provider.Usage{TotalTokens: 1}}

	// Fill cache first so every Set triggers an eviction.
	for i := 0; i < size; i++ {
		c.Set(strconv.Itoa(i), resp)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(strconv.Itoa(size+i), resp)
	}
}

func BenchmarkLRUGet_Hit(b *testing.B) {
	c := NewMemoryCache(1000, time.Hour)
	defer c.Close()
	resp := &provider.ChatResponse{Usage: provider.Usage{TotalTokens: 1}}
	for i := 0; i < 1000; i++ {
		c.Set(strconv.Itoa(i), resp)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(strconv.Itoa(i % 1000))
	}
}

func BenchmarkCacheKey(b *testing.B) {
	req := &provider.ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []provider.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "What is the meaning of life, the universe, and everything? Please answer in detail."},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CacheKey(req)
	}
}
