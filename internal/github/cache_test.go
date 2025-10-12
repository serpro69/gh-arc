package github

import (
	"sync"
	"testing"
	"time"
)

func TestNoOpCache(t *testing.T) {
	cache := &NoOpCache{}

	t.Run("Get always returns not found", func(t *testing.T) {
		value, found := cache.Get("test-key")
		if found {
			t.Errorf("NoOpCache.Get should always return found=false, got true")
		}
		if value != nil {
			t.Errorf("NoOpCache.Get should return nil value, got %v", value)
		}
	})

	t.Run("Set does nothing", func(t *testing.T) {
		// Should not panic
		cache.Set("test-key", "test-value", 1*time.Minute)

		// Verify it's still not found
		_, found := cache.Get("test-key")
		if found {
			t.Errorf("NoOpCache.Set should not store values")
		}
	})

	t.Run("Delete does nothing", func(t *testing.T) {
		// Should not panic
		cache.Delete("test-key")
	})

	t.Run("Clear does nothing", func(t *testing.T) {
		// Should not panic
		cache.Clear()
	})

	t.Run("GetETag returns empty", func(t *testing.T) {
		etag, found := cache.GetETag("test-key")
		if found {
			t.Errorf("NoOpCache.GetETag should return found=false")
		}
		if etag != "" {
			t.Errorf("NoOpCache.GetETag should return empty string, got %s", etag)
		}
	})

	t.Run("SetWithETag does nothing", func(t *testing.T) {
		cache.SetWithETag("test-key", "test-value", "etag-123", 1*time.Minute)

		_, found := cache.Get("test-key")
		if found {
			t.Errorf("NoOpCache.SetWithETag should not store values")
		}
	})

	t.Run("Stats returns empty", func(t *testing.T) {
		stats := cache.Stats()
		if stats.Hits != 0 || stats.Misses != 0 || stats.Size != 0 {
			t.Errorf("NoOpCache.Stats should return zero values")
		}
	})
}

func TestNoOpCacheImplementsCache(t *testing.T) {
	var _ Cache = (*NoOpCache)(nil)
}

func TestMemoryCache(t *testing.T) {
	t.Run("basic set and get", func(t *testing.T) {
		cache := NewMemoryCache(1 * time.Minute)
		defer cache.Stop()

		cache.Set("key1", "value1", 1*time.Minute)

		value, found := cache.Get("key1")
		if !found {
			t.Errorf("expected to find key1")
		}
		if value != "value1" {
			t.Errorf("expected value1, got %v", value)
		}
	})

	t.Run("get non-existent key", func(t *testing.T) {
		cache := NewMemoryCache(1 * time.Minute)
		defer cache.Stop()

		value, found := cache.Get("nonexistent")
		if found {
			t.Errorf("should not find nonexistent key")
		}
		if value != nil {
			t.Errorf("should return nil for nonexistent key")
		}
	})

	t.Run("TTL expiration", func(t *testing.T) {
		cache := NewMemoryCache(10 * time.Millisecond)
		defer cache.Stop()

		// Set with very short TTL
		cache.Set("key1", "value1", 50*time.Millisecond)

		// Should be found immediately
		_, found := cache.Get("key1")
		if !found {
			t.Errorf("key should be found before expiration")
		}

		// Wait for expiration
		time.Sleep(100 * time.Millisecond)

		// Should not be found after expiration
		_, found = cache.Get("key1")
		if found {
			t.Errorf("key should not be found after expiration")
		}
	})

	t.Run("ETag storage and retrieval", func(t *testing.T) {
		cache := NewMemoryCache(1 * time.Minute)
		defer cache.Stop()

		etag := "W/\"abc123\""
		cache.SetWithETag("key1", "value1", etag, 1*time.Minute)

		// Should retrieve the value
		value, found := cache.Get("key1")
		if !found {
			t.Errorf("expected to find key1")
		}
		if value != "value1" {
			t.Errorf("expected value1, got %v", value)
		}

		// Should retrieve the ETag
		retrievedETag, found := cache.GetETag("key1")
		if !found {
			t.Errorf("expected to find ETag for key1")
		}
		if retrievedETag != etag {
			t.Errorf("expected ETag %s, got %s", etag, retrievedETag)
		}
	})

	t.Run("GetETag for expired entry", func(t *testing.T) {
		cache := NewMemoryCache(10 * time.Millisecond)
		defer cache.Stop()

		cache.SetWithETag("key1", "value1", "etag-123", 50*time.Millisecond)

		// Wait for expiration
		time.Sleep(100 * time.Millisecond)

		// Should not find ETag after expiration
		_, found := cache.GetETag("key1")
		if found {
			t.Errorf("should not find ETag after expiration")
		}
	})

	t.Run("Delete removes entry", func(t *testing.T) {
		cache := NewMemoryCache(1 * time.Minute)
		defer cache.Stop()

		cache.Set("key1", "value1", 1*time.Minute)
		cache.Delete("key1")

		_, found := cache.Get("key1")
		if found {
			t.Errorf("key should be deleted")
		}
	})

	t.Run("Clear removes all entries", func(t *testing.T) {
		cache := NewMemoryCache(1 * time.Minute)
		defer cache.Stop()

		cache.Set("key1", "value1", 1*time.Minute)
		cache.Set("key2", "value2", 1*time.Minute)
		cache.Clear()

		_, found1 := cache.Get("key1")
		_, found2 := cache.Get("key2")

		if found1 || found2 {
			t.Errorf("all keys should be cleared")
		}
	})

	t.Run("Stats tracking", func(t *testing.T) {
		cache := NewMemoryCache(1 * time.Minute)
		defer cache.Stop()

		// Set some values
		cache.Set("key1", "value1", 1*time.Minute)
		cache.Set("key2", "value2", 1*time.Minute)

		// Generate hits
		cache.Get("key1")
		cache.Get("key2")

		// Generate misses
		cache.Get("nonexistent1")
		cache.Get("nonexistent2")

		stats := cache.Stats()

		if stats.Hits != 2 {
			t.Errorf("expected 2 hits, got %d", stats.Hits)
		}
		if stats.Misses != 2 {
			t.Errorf("expected 2 misses, got %d", stats.Misses)
		}
		if stats.Size != 2 {
			t.Errorf("expected size 2, got %d", stats.Size)
		}
		if stats.HitRate != 50.0 {
			t.Errorf("expected hit rate 50%%, got %.2f%%", stats.HitRate)
		}
	})

	t.Run("automatic cleanup of expired entries", func(t *testing.T) {
		cache := NewMemoryCache(50 * time.Millisecond)
		defer cache.Stop()

		// Set entries with short TTL
		cache.Set("key1", "value1", 100*time.Millisecond)
		cache.Set("key2", "value2", 100*time.Millisecond)

		// Wait for cleanup to run
		time.Sleep(200 * time.Millisecond)

		// Check stats - entries should be evicted
		stats := cache.Stats()
		if stats.Evictions < 1 {
			t.Errorf("expected at least 1 eviction, got %d", stats.Evictions)
		}
	})

	t.Run("concurrent access", func(t *testing.T) {
		cache := NewMemoryCache(1 * time.Minute)
		defer cache.Stop()

		var wg sync.WaitGroup
		numGoroutines := 100
		numOperations := 100

		// Concurrent writes
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					key := string(rune('a' + (id+j)%26))
					cache.Set(key, id*numOperations+j, 1*time.Minute)
				}
			}(i)
		}

		// Concurrent reads
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					key := string(rune('a' + (id+j)%26))
					cache.Get(key)
				}
			}(i)
		}

		wg.Wait()

		// Should not panic - verify cache is still usable
		cache.Set("final", "value", 1*time.Minute)
		_, found := cache.Get("final")
		if !found {
			t.Errorf("cache should still work after concurrent access")
		}
	})
}

func TestMemoryCacheImplementsCache(t *testing.T) {
	var _ Cache = (*MemoryCache)(nil)
}

func TestGenerateCacheKey(t *testing.T) {
	t.Run("simple key without params", func(t *testing.T) {
		key := GenerateCacheKey("GET", "/repos/owner/repo", nil)
		expected := "GET:/repos/owner/repo"
		if key != expected {
			t.Errorf("expected %s, got %s", expected, key)
		}
	})

	t.Run("key with params", func(t *testing.T) {
		params := map[string]string{
			"page":     "1",
			"per_page": "30",
		}
		key := GenerateCacheKey("GET", "/repos/owner/repo/issues", params)

		// With params, should return a hash
		if key == "GET:/repos/owner/repo/issues" {
			t.Errorf("expected hashed key with params")
		}
		if len(key) != 64 { // SHA256 hex length
			t.Errorf("expected SHA256 hash length, got %d", len(key))
		}
	})

	t.Run("consistent key generation", func(t *testing.T) {
		params := map[string]string{
			"page":     "1",
			"per_page": "30",
		}

		key1 := GenerateCacheKey("GET", "/repos/owner/repo/issues", params)
		key2 := GenerateCacheKey("GET", "/repos/owner/repo/issues", params)

		if key1 != key2 {
			t.Errorf("keys should be consistent for same inputs")
		}
	})

	t.Run("different methods produce different keys", func(t *testing.T) {
		key1 := GenerateCacheKey("GET", "/repos/owner/repo", nil)
		key2 := GenerateCacheKey("POST", "/repos/owner/repo", nil)

		if key1 == key2 {
			t.Errorf("different methods should produce different keys")
		}
	})
}
