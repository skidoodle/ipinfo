package common

import (
	"sync"
	"time"
)

// cachedItem represents a generic item in the cache.
type cachedItem struct {
	data any
	time time.Time
}

// Cache provides a thread-safe, generic caching mechanism with a TTL.
type Cache struct {
	store sync.Map
	ttl   time.Duration
}

// NewCache creates a new generic cache with the specified TTL.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		ttl: ttl,
	}
}

// Set adds a new entry to the cache.
func (c *Cache) Set(key any, data any) {
	c.store.Store(key, cachedItem{
		data: data,
		time: time.Now(),
	})
}

// Get retrieves an entry from the cache.
func (c *Cache) Get(key any) (any, bool) {
	if item, ok := c.store.Load(key); ok {
		cached := item.(cachedItem)
		if time.Since(cached.time) < c.ttl {
			return cached.data, true
		}
		c.store.Delete(key)
	}
	return nil, false
}

// Global cache with a 10-minute TTL.
var cache = NewCache(10 * time.Minute)
