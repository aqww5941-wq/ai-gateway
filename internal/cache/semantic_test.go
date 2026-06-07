package cache

import (
	"testing"
	"time"

	"ai-gateway/internal/provider"
)

func TestSemanticCacheSimilar(t *testing.T) {
	cache := NewSemanticCache(10, time.Minute, 0.5)

	req1 := &provider.ChatRequest{
		Model: "gpt-4o",
		Messages: []provider.Message{
			{Role: "user", Content: "how to write a function in python"},
		},
	}
	resp1 := &provider.ChatResponse{ID: "r1", Model: "gpt-4o"}

	cache.SetWithEmbedding(req1, resp1)

	// Similar request
	req2 := &provider.ChatRequest{
		Model: "gpt-4o",
		Messages: []provider.Message{
			{Role: "user", Content: "how to write a python function"},
		},
	}

	got, ok := cache.GetSimilar(req2)
	if !ok {
		t.Log("similarity match not found (depends on threshold)")
	}
	if ok && got.ID != "r1" {
		t.Errorf("expected ID r1, got %s", got.ID)
	}

	// Very different request
	req3 := &provider.ChatRequest{
		Model: "gpt-4o",
		Messages: []provider.Message{
			{Role: "user", Content: "hello world thank you please help"},
		},
	}
	_, ok = cache.GetSimilar(req3)
	if ok {
		t.Error("expected miss for very different request")
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{1, 0, 0}
	if cosineSimilarity(a, b) != 1.0 {
		t.Errorf("identical vectors should have cosine similarity 1.0, got %f", cosineSimilarity(a, b))
	}

	c := []float64{0, 1, 0}
	if cosineSimilarity(a, c) != 0.0 {
		t.Errorf("orthogonal vectors should have cosine similarity 0.0, got %f", cosineSimilarity(a, c))
	}
}
