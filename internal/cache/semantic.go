package cache

import (
	"math"
	"strings"
	"sync"
	"time"

	"ai-gateway/internal/provider"
)

// SemanticCache wraps MemoryCache with embedding-based similarity matching.
type SemanticCache struct {
	*MemoryCache
	mu         sync.RWMutex
	threshold  float64
	embeddings map[string][]float64
}

func NewSemanticCache(maxSize int, ttl time.Duration, threshold float64) *SemanticCache {
	return &SemanticCache{
		MemoryCache: NewMemoryCache(maxSize, ttl),
		threshold:   threshold,
		embeddings:  make(map[string][]float64),
	}
}

// GetSimilar searches the cache for a semantically similar request.
// Returns the cached response and true if a similar match is found above threshold.
func (c *SemanticCache) GetSimilar(req *provider.ChatRequest) (*provider.ChatResponse, bool) {
	vec := embedRequest(req)

	c.mu.RLock()
	defer c.mu.RUnlock()

	var bestKey string
	var bestScore float64

	for key, cachedVec := range c.embeddings {
		score := cosineSimilarity(vec, cachedVec)
		if score > bestScore && score >= c.threshold {
			bestScore = score
			bestKey = key
		}
	}

	if bestKey != "" {
		return c.MemoryCache.Get(bestKey)
	}
	return nil, false
}

// SetWithEmbedding stores the response and its embedding vector.
func (c *SemanticCache) SetWithEmbedding(req *provider.ChatRequest, resp *provider.ChatResponse) {
	key := CacheKey(req)
	vec := embedRequest(req)

	c.MemoryCache.Set(key, resp)

	c.mu.Lock()
	c.embeddings[key] = vec
	c.mu.Unlock()
}

// embedRequest creates a simple bag-of-words vector from the request messages.
// Uses a fixed vocabulary of common coding/LLM terms for lightweight embedding.
func embedRequest(req *provider.ChatRequest) []float64 {
	vocab := []string{
		"function", "class", "code", "implement", "write", "create", "build",
		"explain", "what", "how", "why", "when", "where",
		"error", "bug", "fix", "debug", "test", "compile", "runtime",
		"python", "javascript", "go", "rust", "java", "typescript",
		"api", "http", "json", "database", "sql", "server", "client",
		"async", "await", "promise", "goroutine", "channel", "thread",
		"optimize", "performance", "memory", "cpu", "cache", "concurrent",
		"return", "value", "type", "interface", "struct", "method",
		"hello", "hi", "thank", "please", "help", "need", "want",
	}

	// Concatenate all message contents
	var sb strings.Builder
	for _, m := range req.Messages {
		sb.WriteString(strings.ToLower(m.Content))
		sb.WriteByte(' ')
	}
	text := sb.String()

	// TF vector
	vec := make([]float64, len(vocab))
	total := 0
	for i, word := range vocab {
		count := strings.Count(text, word)
		vec[i] = float64(count)
		total += count
	}

	// Normalize
	if total > 0 {
		for i := range vec {
			vec[i] /= float64(total)
		}
	}

	return vec
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
