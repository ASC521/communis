package cache

import (
	"sync"
	"time"
)

// item represents a cache item with a value and an expiration time.
type item[V any] struct {
	value  V
	expiry time.Time
}

// isExpired checks if the cache item has expired.
func (i item[V]) isExpired() bool {
	return time.Now().After(i.expiry)
}

// TTLCache is a generic cache implementation with support for time-to-live (TTL) expiration.
type TTLCache[K comparable, V any] struct {
	items    map[K]item[V]
	mu       sync.Mutex
	onDelete func(K, V)
}

func NewTTL[K comparable, V any](onDelete func(K, V)) *TTLCache[K, V] {
	cache := &TTLCache[K, V]{
		items:    make(map[K]item[V]),
		onDelete: onDelete,
	}

	go cache.removeExpired()

	return cache
}

func (c *TTLCache[K, V]) removeExpired() {
	for range time.Tick(60 * time.Second) {
		c.mu.Lock()

		for key, item := range c.items {
			if !item.isExpired() {
				continue
			}

			if c.onDelete != nil {
				c.onDelete(key, item.value)
			}
			delete(c.items, key)
		}
		c.mu.Unlock()
	}
}

// Set adds a new items to the cache with the specified key, value, and time-to-live (TTL).
func (c *TTLCache[K, V]) Set(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = item[V]{
		value:  value,
		expiry: time.Now().Add(ttl),
	}
}

// Get retrieves the value associated with the given key from the cache.
func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, found := c.items[key]
	if !found {
		return item.value, false
	}

	if item.isExpired() {
		if c.onDelete != nil {
			c.onDelete(key, item.value)
		}
		delete(c.items, key)
		return item.value, false
	}

	return item.value, true
}

// Remove moves the item with the specified key from the cache.
func (c *TTLCache[K, V]) Remove(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, found := c.items[key]
	if !found {
		return
	}

	if c.onDelete != nil {
		c.onDelete(key, item.value)
	}
	delete(c.items, key)
}

// Shutdown removes all items and delete the cache
func (c *TTLCache[K, V]) Shutdown() {
	for key, item := range c.items {
		if c.onDelete != nil {
			c.onDelete(key, item.value)
		}
	}

	c.items = make(map[K]item[V])
}
