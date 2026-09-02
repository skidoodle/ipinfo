package cache

import (
	"sync"
	"time"
)

type item[V any] struct {
	value     V
	expiresAt time.Time
}

// Cache provides a thread-safe, generic in-memory cache with TTL and active eviction.
type Cache[K comparable, V any] struct {
	mu      sync.RWMutex
	items   map[K]item[V]
	ttl     time.Duration
	stopCh  chan struct{}
	closeMu sync.Mutex
	closed  bool
}

// New creates a new Cache with the specified TTL and eviction interval.
// If cleanupInterval <= 0, cleanupInterval defaults to ttl.
func New[K comparable, V any](ttl time.Duration, cleanupInterval ...time.Duration) *Cache[K, V] {
	interval := ttl
	if len(cleanupInterval) > 0 && cleanupInterval[0] > 0 {
		interval = cleanupInterval[0]
	}

	c := &Cache[K, V]{
		items:  make(map[K]item[V]),
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}

	go c.janitor(interval)
	return c
}

// Set adds or overwrites an item in the cache with the default TTL.
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = item[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Get retrieves an item from the cache. Returns false if not found or expired.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	it, found := c.items[key]
	c.mu.RUnlock()

	if !found {
		var zero V
		return zero, false
	}

	if time.Now().After(it.expiresAt) {
		// Clean up expired key lazily
		c.mu.Lock()
		// Recheck in case it was updated between RUnlock and Lock
		if cur, ok := c.items[key]; ok && time.Now().After(cur.expiresAt) {
			delete(c.items, key)
		}
		c.mu.Unlock()

		var zero V
		return zero, false
	}

	return it.value, true
}

// Delete removes an item from the cache.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// Len returns the current number of items in the cache (including potentially expired ones).
func (c *Cache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// PurgeExpired actively removes all expired entries from the cache.
func (c *Cache[K, V]) PurgeExpired() {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	for k, it := range c.items {
		if now.After(it.expiresAt) {
			delete(c.items, k)
		}
	}
}

// Stop stops the background eviction janitor.
func (c *Cache[K, V]) Stop() {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.stopCh)
	}
}

func (c *Cache[K, V]) janitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.PurgeExpired()
		case <-c.stopCh:
			return
		}
	}
}
