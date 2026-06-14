package cache

import (
	"testing"
	"time"
)

func TestLRUCache_SetAndGet(t *testing.T) {
	cache := NewLRUCache(1*time.Second, 100)
	defer cache.Close()

	// Test Set and Get
	cache.Set("key1", "value1")
	value, ok := cache.Get("key1")
	if !ok {
		t.Error("expected to find key1")
	}
	if value != "value1" {
		t.Errorf("expected value1, got %v", value)
	}

	// Test non-existent key
	_, ok = cache.Get("key2")
	if ok {
		t.Error("expected not to find key2")
	}
}

func TestLRUCache_Expiration(t *testing.T) {
	cache := NewLRUCache(100*time.Millisecond, 100)
	defer cache.Close()

	// Set with short TTL
	cache.SetWithTTL("key1", "value1", 50*time.Millisecond)

	// Should exist immediately
	_, ok := cache.Get("key1")
	if !ok {
		t.Error("expected to find key1")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should not exist after expiration
	_, ok = cache.Get("key1")
	if ok {
		t.Error("expected key1 to be expired")
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	cache := NewLRUCache(1*time.Second, 3)
	defer cache.Close()

	// Add 3 items
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	if cache.Size() != 3 {
		t.Errorf("expected size 3, got %d", cache.Size())
	}

	// Add 4th item - should evict oldest
	cache.Set("key4", "value4")

	if cache.Size() != 3 {
		t.Errorf("expected size 3 after eviction, got %d", cache.Size())
	}

	// key1 should be evicted
	_, ok := cache.Get("key1")
	if ok {
		t.Error("expected key1 to be evicted")
	}

	// key4 should exist
	_, ok = cache.Get("key4")
	if !ok {
		t.Error("expected key4 to exist")
	}
}

func TestLRUCache_LRUBehavior(t *testing.T) {
	cache := NewLRUCache(1*time.Second, 3)
	defer cache.Close()

	// Add 3 items
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// Access key1 (makes it most recently used)
	cache.Get("key1")

	// Add 4th item - should evict key2 (oldest)
	cache.Set("key4", "value4")

	// key1 should still exist (was recently accessed)
	_, ok := cache.Get("key1")
	if !ok {
		t.Error("expected key1 to exist (was recently accessed)")
	}

	// key2 should be evicted
	_, ok = cache.Get("key2")
	if ok {
		t.Error("expected key2 to be evicted")
	}
}

func TestLRUCache_Delete(t *testing.T) {
	cache := NewLRUCache(1*time.Second, 100)
	defer cache.Close()

	cache.Set("key1", "value1")
	cache.Delete("key1")

	_, ok := cache.Get("key1")
	if ok {
		t.Error("expected key1 to be deleted")
	}
}

func TestLRUCache_Clear(t *testing.T) {
	cache := NewLRUCache(1*time.Second, 100)
	defer cache.Close()

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	if cache.Size() != 2 {
		t.Errorf("expected size 2, got %d", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("expected size 0, got %d", cache.Size())
	}
}

func TestLRUCache_Stats(t *testing.T) {
	cache := NewLRUCache(1*time.Second, 100)
	defer cache.Close()

	// Generate some hits and misses
	cache.Set("key1", "value1")
	cache.Get("key1") // hit
	cache.Get("key2") // miss
	cache.Get("key1") // hit

	stats := cache.Stats()
	if stats.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.Size != 1 {
		t.Errorf("expected size 1, got %d", stats.Size)
	}

	hitRate := stats.HitRate()
	if hitRate < 0.66 || hitRate > 0.67 {
		t.Errorf("expected hit rate ~0.67, got %f", hitRate)
	}
}

func TestTypedLRUCache(t *testing.T) {
	cache := NewTypedLRUCache[string](1*time.Second, 100)
	defer cache.Close()

	// Set string value
	cache.Set("key1", "value1")

	// Get string value
	value, ok := cache.Get("key1")
	if !ok {
		t.Error("expected to find key1")
	}
	if value != "value1" {
		t.Errorf("expected value1, got %v", value)
	}

	// Get non-existent key
	_, ok = cache.Get("key2")
	if ok {
		t.Error("expected not to find key2")
	}
}
