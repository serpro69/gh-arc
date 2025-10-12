package github

import "time"

// Cache defines the interface for caching GitHub API responses
// This will be implemented in subtask 2.4
type Cache interface {
	// Get retrieves a cached value by key
	Get(key string) (value interface{}, found bool)

	// Set stores a value in the cache with a TTL
	Set(key string, value interface{}, ttl time.Duration)

	// Delete removes a value from the cache
	Delete(key string)

	// Clear removes all values from the cache
	Clear()
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
