package cache

import (
	"testing"
	"time"

	"ai-gateway/internal/provider"
)

func TestCacheKey(t *testing.T) {
	req1 := &provider.ChatRequest{
		Model: "gpt-4o",
		Messages: []provider.Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
		},
	}
	req2 := &provider.ChatRequest{
		Model: "gpt-4o",
		Messages: []provider.Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
		},
	}

	k1 := CacheKey(req1)
	k2 := CacheKey(req2)
	if k1 != k2 {
		t.Errorf("same requests should have same key: %q != %q", k1, k2)
	}

	req3 := &provider.ChatRequest{
		Model: "gpt-4o",
		Messages: []provider.Message{
			{Role: "user", Content: "Different message"},
		},
	}
	if CacheKey(req1) == CacheKey(req3) {
		t.Error("different requests should have different keys")
	}
}

func TestMemoryCacheGetSet(t *testing.T) {
	cache := NewMemoryCache(10, time.Minute)

	key := "test-key"
	resp := &provider.ChatResponse{
		ID:    "resp-1",
		Model: "gpt-4o",
		Choices: []provider.Choice{
			{Index: 0, Message: provider.Message{Role: "assistant", Content: "Hi!"}},
		},
	}

	cache.Set(key, resp)
	got, ok := cache.Get(key)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.ID != resp.ID {
		t.Errorf("expected ID %q, got %q", resp.ID, got.ID)
	}
}

func TestMemoryCacheExpiry(t *testing.T) {
	cache := NewMemoryCache(10, 1*time.Millisecond)

	key := "expires-soon"
	cache.Set(key, &provider.ChatResponse{ID: "x"})

	time.Sleep(5 * time.Millisecond)

	_, ok := cache.Get(key)
	if ok {
		t.Error("expected cache miss after TTL")
	}
}

func TestMemoryCacheEviction(t *testing.T) {
	cache := NewMemoryCache(2, time.Hour)

	cache.Set("k1", &provider.ChatResponse{ID: "1"})
	cache.Set("k2", &provider.ChatResponse{ID: "2"})
	cache.Set("k3", &provider.ChatResponse{ID: "3"})

	// One of the first two should be evicted
	hits := 0
	for _, k := range []string{"k1", "k2", "k3"} {
		if _, ok := cache.Get(k); ok {
			hits++
		}
	}
	if hits != 2 {
		t.Errorf("expected 2 entries after eviction, got %d", hits)
	}
}
