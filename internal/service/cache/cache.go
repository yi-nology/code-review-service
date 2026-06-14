package cache

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"
)

// CacheStats contains cache statistics
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Size      int
}

// HitRate returns the cache hit rate
func (s *CacheStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total)
}

// CacheItem represents a cached item
type CacheItem struct {
	Key       string
	Value     interface{}
	ExpiresAt time.Time
	Element   *list.Element
}

// IsExpired checks if the item is expired
func (c *CacheItem) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// LRUCache provides an in-memory LRU cache with TTL support
type LRUCache struct {
	items    map[string]*CacheItem
	lru      *list.List
	mu       sync.RWMutex
	ttl      time.Duration
	maxItems int
	stats    CacheStats
	stop     chan struct{}
}

// NewLRUCache creates a new LRU cache with the specified TTL and max items
func NewLRUCache(ttl time.Duration, maxItems int) *LRUCache {
	if maxItems <= 0 {
		maxItems = 1000
	}

	c := &LRUCache{
		items:    make(map[string]*CacheItem),
		lru:      list.New(),
		ttl:      ttl,
		maxItems: maxItems,
		stop:     make(chan struct{}),
	}

	// Start cleanup goroutine
	go c.cleanup()

	return c
}

// Get retrieves an item from the cache
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		atomic.AddInt64(&c.stats.Misses, 1)
		return nil, false
	}

	if item.IsExpired() {
		c.removeItem(item)
		atomic.AddInt64(&c.stats.Misses, 1)
		return nil, false
	}

	// Move to front (most recently used)
	c.lru.MoveToFront(item.Element)
	atomic.AddInt64(&c.stats.Hits, 1)

	return item.Value, true
}

// Set adds an item to the cache with the default TTL
func (c *LRUCache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.ttl)
}

// SetWithTTL adds an item to the cache with a custom TTL
func (c *LRUCache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing item
	if item, ok := c.items[key]; ok {
		item.Value = value
		item.ExpiresAt = time.Now().Add(ttl)
		c.lru.MoveToFront(item.Element)
		return
	}

	// Evict if at capacity
	for c.lru.Len() >= c.maxItems {
		c.evictOldest()
	}

	// Add new item
	item := &CacheItem{
		Key:       key,
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
	element := c.lru.PushFront(item)
	item.Element = element
	c.items[key] = item
}

// Delete removes an item from the cache
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, ok := c.items[key]; ok {
		c.removeItem(item)
	}
}

// Clear removes all items from the cache
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*CacheItem)
	c.lru.Init()
}

// Size returns the number of items in the cache
func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lru.Len()
}

// Stats returns cache statistics
func (c *LRUCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		Hits:      atomic.LoadInt64(&c.stats.Hits),
		Misses:    atomic.LoadInt64(&c.stats.Misses),
		Evictions: atomic.LoadInt64(&c.stats.Evictions),
		Size:      c.lru.Len(),
	}
	return stats
}

// removeItem removes an item from the cache (must be called with lock held)
func (c *LRUCache) removeItem(item *CacheItem) {
	c.lru.Remove(item.Element)
	delete(c.items, item.Key)
}

// evictOldest evicts the oldest item from the cache (must be called with lock held)
func (c *LRUCache) evictOldest() {
	element := c.lru.Back()
	if element == nil {
		return
	}

	item := element.Value.(*CacheItem)
	c.removeItem(item)
	atomic.AddInt64(&c.stats.Evictions, 1)
}

// cleanup removes expired items periodically
func (c *LRUCache) cleanup() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.removeExpired()
		case <-c.stop:
			return
		}
	}
}

// removeExpired removes all expired items
func (c *LRUCache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.items {
		if now.After(item.ExpiresAt) {
			c.removeItem(item)
			delete(c.items, key)
		}
	}
}

// Close stops the cleanup goroutine
func (c *LRUCache) Close() {
	close(c.stop)
}

// TypedLRUCache provides type-safe LRU caching
type TypedLRUCache[T any] struct {
	cache *LRUCache
}

// NewTypedLRUCache creates a new typed LRU cache
func NewTypedLRUCache[T any](ttl time.Duration, maxItems int) *TypedLRUCache[T] {
	return &TypedLRUCache[T]{
		cache: NewLRUCache(ttl, maxItems),
	}
}

// Get retrieves a typed item from the cache
func (c *TypedLRUCache[T]) Get(key string) (T, bool) {
	value, ok := c.cache.Get(key)
	if !ok {
		var zero T
		return zero, false
	}

	typed, ok := value.(T)
	if !ok {
		var zero T
		return zero, false
	}

	return typed, true
}

// Set adds a typed item to the cache
func (c *TypedLRUCache[T]) Set(key string, value T) {
	c.cache.Set(key, value)
}

// Delete removes an item from the cache
func (c *TypedLRUCache[T]) Delete(key string) {
	c.cache.Delete(key)
}

// Size returns the number of items in the cache
func (c *TypedLRUCache[T]) Size() int {
	return c.cache.Size()
}

// Stats returns cache statistics
func (c *TypedLRUCache[T]) Stats() CacheStats {
	return c.cache.Stats()
}

// Close stops the cache
func (c *TypedLRUCache[T]) Close() {
	c.cache.Close()
}

// CacheConfig contains cache configuration
type CacheConfig struct {
	DefaultTTL      time.Duration
	MaxItems        int
	CleanupInterval time.Duration
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		DefaultTTL:      5 * time.Minute,
		MaxItems:        1000,
		CleanupInterval: 1 * time.Minute,
	}
}
