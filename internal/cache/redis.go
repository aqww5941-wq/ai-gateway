package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ai-gateway/internal/provider"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
	prefix string
}

func NewRedisCache(addr, password string, db int, ttl time.Duration) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &RedisCache{
		client: client,
		ttl:    ttl,
		prefix: "gateway:cache:",
	}, nil
}

func (c *RedisCache) Get(key string) (*provider.ChatResponse, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	data, err := c.client.Get(ctx, c.prefix+key).Bytes()
	if err != nil {
		return nil, false
	}

	var resp provider.ChatResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, false
	}
	return &resp, true
}

func (c *RedisCache) Set(key string, resp *provider.ChatResponse) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	data, err := json.Marshal(resp)
	if err != nil {
		return
	}

	c.client.Set(ctx, c.prefix+key, data, c.ttl)
}
