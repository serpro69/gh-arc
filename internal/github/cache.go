package github

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Cache defines the interface for caching GitHub API responses
type Cache interface {
	// Get retrieves a cached value by key
	Get(key string) (value interface{}, found bool)

	// Set stores a value in the cache with a TTL
	Set(key string, value interface{}, ttl time.Duration)

	// Delete removes a value from the cache
	Delete(key string)

	// Clear removes all values from the cache
	Clear()

	// GetETag retrieves the ETag for a cached key
	GetETag(key string) (etag string, found bool)

	// SetWithETag stores a value with its ETag
	SetWithETag(key string, value interface{}, etag string, ttl time.Duration)

	// Stats returns cache statistics
	Stats() CacheStats
}

// CacheStats holds statistics about cache performance
type CacheStats struct {
	Hits          int64 // Number of cache hits
	Misses        int64 // Number of cache misses
	Evictions     int64 // Number of evictions due to TTL expiration
	Size          int   // Current number of entries
	HitRate       float64 // Hit rate percentage (hits / (hits + misses))
}

// cacheEntry represents a single cached item with metadata
type cacheEntry struct {
	value      interface{}
	etag       string
	expiresAt  time.Time
	lastAccess time.Time
}

// isExpired checks if the entry has expired
func (e *cacheEntry) isExpired() bool {
	return time.Now().After(e.expiresAt)
}

// MemoryCache is a thread-safe in-memory cache with TTL and ETag support
type MemoryCache struct {
	entries sync.Map // map[string]*cacheEntry
	hits    atomic.Int64
	misses  atomic.Int64
	evictions atomic.Int64

	// cleanupInterval defines how often to check for expired entries
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
	cleanupOnce     sync.Once
}

// NewMemoryCache creates a new in-memory cache with automatic cleanup
func NewMemoryCache(cleanupInterval time.Duration) *MemoryCache {
	if cleanupInterval == 0 {
		cleanupInterval = 1 * time.Minute // Default cleanup interval
	}

	cache := &MemoryCache{
		cleanupInterval: cleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// Start background cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// cleanupLoop periodically removes expired entries
func (m *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.removeExpired()
		case <-m.stopCleanup:
			return
		}
	}
}

// removeExpired removes all expired entries from the cache
func (m *MemoryCache) removeExpired() {
	m.entries.Range(func(key, value interface{}) bool {
		entry := value.(*cacheEntry)
		if entry.isExpired() {
			m.entries.Delete(key)
			m.evictions.Add(1)
		}
		return true
	})
}

// Stop stops the background cleanup goroutine
// Should be called when the cache is no longer needed
func (m *MemoryCache) Stop() {
	m.cleanupOnce.Do(func() {
		close(m.stopCleanup)
	})
}

// Get retrieves a cached value by key
func (m *MemoryCache) Get(key string) (interface{}, bool) {
	value, found := m.entries.Load(key)
	if !found {
		m.misses.Add(1)
		return nil, false
	}

	entry := value.(*cacheEntry)

	// Check if expired
	if entry.isExpired() {
		m.entries.Delete(key)
		m.evictions.Add(1)
		m.misses.Add(1)
		return nil, false
	}

	// Update last access time
	entry.lastAccess = time.Now()

	m.hits.Add(1)
	return entry.value, true
}

// Set stores a value in the cache with a TTL
func (m *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	m.SetWithETag(key, value, "", ttl)
}

// GetETag retrieves the ETag for a cached key
func (m *MemoryCache) GetETag(key string) (string, bool) {
	value, found := m.entries.Load(key)
	if !found {
		return "", false
	}

	entry := value.(*cacheEntry)

	// Check if expired
	if entry.isExpired() {
		m.entries.Delete(key)
		m.evictions.Add(1)
		return "", false
	}

	return entry.etag, true
}

// SetWithETag stores a value with its ETag
func (m *MemoryCache) SetWithETag(key string, value interface{}, etag string, ttl time.Duration) {
	entry := &cacheEntry{
		value:      value,
		etag:       etag,
		expiresAt:  time.Now().Add(ttl),
		lastAccess: time.Now(),
	}

	m.entries.Store(key, entry)
}

// Delete removes a value from the cache
func (m *MemoryCache) Delete(key string) {
	m.entries.Delete(key)
}

// Clear removes all values from the cache
func (m *MemoryCache) Clear() {
	m.entries.Range(func(key, value interface{}) bool {
		m.entries.Delete(key)
		return true
	})

	// Reset statistics
	m.hits.Store(0)
	m.misses.Store(0)
	m.evictions.Store(0)
}

// Stats returns cache statistics
func (m *MemoryCache) Stats() CacheStats {
	hits := m.hits.Load()
	misses := m.misses.Load()
	evictions := m.evictions.Load()

	// Count current entries
	size := 0
	m.entries.Range(func(key, value interface{}) bool {
		size++
		return true
	})

	// Calculate hit rate
	var hitRate float64
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return CacheStats{
		Hits:      hits,
		Misses:    misses,
		Evictions: evictions,
		Size:      size,
		HitRate:   hitRate,
	}
}

// GenerateCacheKey creates a cache key from request parameters
func GenerateCacheKey(method, path string, params map[string]string) string {
	// Create a stable key by concatenating method, path, and sorted params
	key := fmt.Sprintf("%s:%s", method, path)

	// Add parameters if present
	if len(params) > 0 {
		// Sort keys to ensure consistent ordering
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}

		// Use a simple sort for consistency
		// Note: could use sort.Strings() but avoiding import for this simple case
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[i] > keys[j] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}

		// Use a hash to keep keys manageable
		hash := sha256.New()
		hash.Write([]byte(key))
		for _, k := range keys {
			hash.Write([]byte(fmt.Sprintf("%s=%s", k, params[k])))
		}
		return hex.EncodeToString(hash.Sum(nil))
	}

	return key
}

// NoOpCache is a cache implementation that does nothing
// Used when caching is disabled
type NoOpCache struct{}

// Get always returns not found
func (n *NoOpCache) Get(key string) (interface{}, bool) {
	return nil, false
}

// Set does nothing
func (n *NoOpCache) Set(key string, value interface{}, ttl time.Duration) {}

// Delete does nothing
func (n *NoOpCache) Delete(key string) {}

// Clear does nothing
func (n *NoOpCache) Clear() {}

// GetETag always returns empty string and false
func (n *NoOpCache) GetETag(key string) (string, bool) {
	return "", false
}

// SetWithETag does nothing
func (n *NoOpCache) SetWithETag(key string, value interface{}, etag string, ttl time.Duration) {}

// Stats returns empty statistics
func (n *NoOpCache) Stats() CacheStats {
	return CacheStats{}
}
