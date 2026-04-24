package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type entry struct {
	data      any
	expiresAt time.Time
}

type Cache struct {
	mu    sync.RWMutex
	items map[string]entry
	ttl   time.Duration
	redis *redis.Client
}

func New(ttl time.Duration) (*Cache, error) {
	address := os.Getenv("REDIS_URL")
	opt, err := redis.ParseURL(address)

	if err != nil {
		return nil, fmt.Errorf("invalid redis url: %w", err)
	}

	rdb := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	log.Println("redis connected")

	return &Cache{
		items: make(map[string]entry),
		ttl:   ttl,
		redis: rdb,
	}, nil
}

func (c *Cache) Get(key string) (any, bool, error) {
	// Check in-memory cache first
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()
	if ok && time.Now().Before(e.expiresAt) {
		log.Printf("cache hit (memory): %s", key)
		return e.data, true, nil
	}

	// Fall back to Redis
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	val, err := c.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis get %s: %w", key, err)
	}

	var data any
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, false, fmt.Errorf("redis unmarshal %s: %w", key, err)
	}

	// Populate in-memory cache
	c.mu.Lock()
	c.items[key] = entry{data: data, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	log.Printf("cache hit (redis): %s", key)
	return data, true, nil
}

func (c *Cache) Set(key string, data any) error {
	c.mu.Lock()
	c.items[key] = entry{data: data, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()

	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("redis marshal %s: %w", key, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := c.redis.Set(ctx, key, b, c.ttl).Err(); err != nil {
		return fmt.Errorf("redis set %s: %w", key, err)
	}
	return nil
}

func (c *Cache) Delete(key string) error {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := c.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis delete %s: %w", key, err)
	}
	return nil
}
