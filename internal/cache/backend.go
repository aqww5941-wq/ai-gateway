package cache

import "ai-gateway/internal/provider"

type CacheBackend interface {
	Get(key string) (*provider.ChatResponse, bool)
	Set(key string, resp *provider.ChatResponse)
}
