package cache

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"ai-gateway/internal/provider"
)

// MemoryCache is a thread-safe LRU cache with TTL.
// All hot-path operations (Get/Set) are O(1) using a doubly-linked list +
// hash map. A background sweeper evicts expired entries to keep memory bounded
// even when keys are written once and never read again.
type MemoryCache struct {
	mu       sync.Mutex
	entries  map[string]*list.Element
	lru      *list.List
	maxSize  int
	ttl      time.Duration
	stopCh   chan struct{}
	stopOnce sync.Once
}

type cacheEntry struct {
	key       string
	response  *provider.ChatResponse
	expiresAt time.Time
}

func NewMemoryCache(maxSize int, ttl time.Duration) *MemoryCache {
	c := &MemoryCache{
		entries: make(map[string]*list.Element, maxSize),
		lru:     list.New(),
		maxSize: maxSize,
		ttl:     ttl,
		stopCh:  make(chan struct{}),
	}
	// Sweep expired entries every TTL/2, capped to a sane interval.
	sweepEvery := ttl / 2
	if sweepEvery < time.Second {
		sweepEvery = time.Second
	}
	if sweepEvery > 5*time.Minute {
		sweepEvery = 5 * time.Minute
	}
	go c.sweepLoop(sweepEvery)
	return c
}

// Close stops the background sweeper. Safe to call multiple times.
func (c *MemoryCache) Close() {
	c.stopOnce.Do(func() { close(c.stopCh) })
}

func CacheKey(req *provider.ChatRequest) string {
	h := sha256.New()
	h.Write([]byte(req.Model))
	// Avoid the fmt.Sprintf allocation on every cache lookup.
	tempBuf := strconv.AppendFloat(nil, ptrFloat(req.Temperature), 'f', 4, 64)
	h.Write(tempBuf)
	for _, m := range req.Messages {
		h.Write([]byte(m.Role))
		h.Write([]byte(m.Content))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func ptrFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func (c *MemoryCache) Get(key string) (*provider.ChatResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	entry := elem.Value.(*cacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.removeElement(elem)
		return nil, false
	}
	// Move to front: mark as most-recently-used.
	c.lru.MoveToFront(elem)
	return entry.response, true
}

func (c *MemoryCache) Set(key string, resp *provider.ChatResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.entries[key]; ok {
		entry := elem.Value.(*cacheEntry)
		entry.response = resp
		entry.expiresAt = time.Now().Add(c.ttl)
		c.lru.MoveToFront(elem)
		return
	}

	if c.maxSize > 0 && c.lru.Len() >= c.maxSize {
		// Evict the least-recently-used entry.
		if oldest := c.lru.Back(); oldest != nil {
			c.removeElement(oldest)
		}
	}

	entry := &cacheEntry{
		key:       key,
		response:  resp,
		expiresAt: time.Now().Add(c.ttl),
	}
	elem := c.lru.PushFront(entry)
	c.entries[key] = elem
}

func (c *MemoryCache) removeElement(elem *list.Element) {
	c.lru.Remove(elem)
	entry := elem.Value.(*cacheEntry)
	delete(c.entries, entry.key)
}

func (c *MemoryCache) sweepLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.sweepExpired()
		}
	}
}

func (c *MemoryCache) sweepExpired() {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	// Walk from oldest to newest; stop at the first non-expired entry
	// because TTL is uniform — anything newer cannot be older.
	for elem := c.lru.Back(); elem != nil; {
		entry := elem.Value.(*cacheEntry)
		prev := elem.Prev()
		if now.After(entry.expiresAt) {
			c.removeElement(elem)
		} else {
			break
		}
		elem = prev
	}
}

func (c *MemoryCache) MarshalJSONResponse(resp *provider.ChatResponse) ([]byte, error) {
	return json.Marshal(resp)
}

func (c *MemoryCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lru.Len()
}

type CacheEntryInfo struct {
	Key        string
	ExpiresAt  time.Time
	TokenCount int
}

func (c *MemoryCache) Info() []CacheEntryInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	infos := make([]CacheEntryInfo, 0, c.lru.Len())
	for elem := c.lru.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*cacheEntry)
		infos = append(infos, CacheEntryInfo{
			Key:        entry.key[:16],
			ExpiresAt:  entry.expiresAt,
			TokenCount: entry.response.Usage.TotalTokens,
		})
	}
	return infos
}
