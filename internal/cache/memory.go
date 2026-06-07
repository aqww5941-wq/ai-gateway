package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"ai-gateway/internal/provider"
)

type MemoryCache struct {
	mu       sync.RWMutex
	entries  map[string]*cacheEntry
	maxSize  int
	ttl      time.Duration
}

type cacheEntry struct {
	response  *provider.ChatResponse
	expiresAt time.Time
}

func NewMemoryCache(maxSize int, ttl time.Duration) *MemoryCache {
	return &MemoryCache{
		entries: make(map[string]*cacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

func CacheKey(req *provider.ChatRequest) string {
	h := sha256.New()
	h.Write([]byte(req.Model))
	h.Write([]byte(fmt.Sprintf("%.4f", ptrFloat(req.Temperature))))
	for _, m := range req.Messages {
		h.Write([]byte(m.Role))
		h.Write([]byte(m.Content))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func ptrFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func (c *MemoryCache) Get(key string) (*provider.ChatResponse, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.response, true
}

func (c *MemoryCache) Set(key string, resp *provider.ChatResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxSize {
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}

	c.entries[key] = &cacheEntry{
		response:  resp,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *MemoryCache) MarshalJSONResponse(resp *provider.ChatResponse) ([]byte, error) {
	return json.Marshal(resp)
}

func (c *MemoryCache) Mu() *sync.RWMutex { return &c.mu }

func (c *MemoryCache) Entries() map[string]*cacheEntry { return c.entries }

type CacheEntryInfo struct {
	Key        string
	ExpiresAt  time.Time
	TokenCount int
}

func (c *MemoryCache) Info() []CacheEntryInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	infos := make([]CacheEntryInfo, 0, len(c.entries))
	for k, e := range c.entries {
		infos = append(infos, CacheEntryInfo{
			Key:        k[:16],
			ExpiresAt:  e.expiresAt,
			TokenCount: e.response.Usage.TotalTokens,
		})
	}
	return infos
}
